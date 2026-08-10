#define FUSE_USE_VERSION 318

#ifdef RENOVATE
int mount_filesystem(const char *mount, int debug) {
    return 0;
}
#else

#include <fuse_lowlevel.h>
#include <stdlib.h>
#include <stdio.h>
#include <string.h>
#include <errno.h>
#include <unistd.h>
#include <sys/time.h>
#include <sys/mount.h>
#include <err.h>
#include "helpers.h"
#include "enabled.h"
#ifdef GO_CGO_BUILD
#include "_cgo_export.h"
#endif

// Taken from https://github.com/libfuse/libfuse/blob/9d63190c9c56a24e1095144e8ffaf66124fa0550/example/passthrough_ll.c#L59-L60
static_assert(sizeof(fuse_ino_t) >= sizeof(uintptr_t), "fuse_ino_t too small to hold uintptr_t values!");

const int MAX_READ = 1 << 20;
const double FUSE_TIMEOUT = 3600.0;

// node_from_ino converts a FUSE inode to a node pointer
static node_t *node_from_ino(fuse_req_t req, fuse_ino_t ino) {
    if (ino == 0) {
        fuse_reply_err(req, ENOENT);

        return NULL;
    }
    if (ino == 1) {
        fs_state_t *state = fuse_req_userdata(req);

        return state->root;
    }

    return (node_t *)(uintptr_t)ino;
}

static void fill_attr(const struct fuse_ctx *ctx, struct stat *stbuf, const node_t *node) {
    *stbuf = node->stat;
    stbuf->st_uid = ctx->uid;
    stbuf->st_gid = ctx->gid;
#if defined(__linux__)
    stbuf->st_atim = node->last_modified;
    stbuf->st_ctim = node->last_modified;
    stbuf->st_mtim = node->last_modified;
#elif defined(__APPLE__)
    stbuf->st_birthtimespec = node->last_modified;
    stbuf->st_atimespec     = node->last_modified;
    stbuf->st_ctimespec     = node->last_modified;
    stbuf->st_mtimespec     = node->last_modified;
#endif
}

static void s3_init(void *userdata, struct fuse_conn_info *conn) {
#if defined(__linux__)
    conn->max_read      = MAX_READ;
    conn->max_readahead = MAX_READ;
#endif

#ifdef FUSE_CONN_FLAG_SINGLE_ISSUER
    fuse_set_conn_flag(conn, FUSE_CONN_FLAG_SINGLE_ISSUER);
#endif
}

static void s3_destroy(void *userdata) {
    fs_state_t *state = (fs_state_t *) userdata;
    pthread_rwlock_wrlock(&state->lock);
    free_nodes(state->root);
    ForgetAllForgotten();
    pthread_rwlock_unlock(&state->lock);
}

static void s3_lookup(fuse_req_t req, fuse_ino_t parent, const char *name) {
    fs_state_t *state = fuse_req_userdata(req);
    pthread_rwlock_rdlock(&state->lock);

    node_t *parent_node = node_from_ino(req, parent);
    if (!parent_node) {
        pthread_rwlock_unlock(&state->lock);

        return;
    }
    node_t *child = find_child_node(parent_node, name);
    if (!child) {
        pthread_rwlock_unlock(&state->lock);
        fuse_reply_err(req, ENOENT);

        return;
    }
    if (child->offset == -2) {
        pthread_rwlock_unlock(&state->lock);
        fuse_reply_err(req, ESTALE);

        return;
    }
    child->lookups++;

    struct fuse_entry_param e = {0};
    e.ino           = child->stat.st_ino;
    e.entry_timeout = child->offset < 0 ? 0.0 : FUSE_TIMEOUT;
    e.attr_timeout  = child->offset < 0 ? 0.0 : FUSE_TIMEOUT;
    fill_attr(fuse_req_ctx(req), &e.attr, child);
    pthread_rwlock_unlock(&state->lock);
    fuse_reply_entry(req, &e);
}

static void s3_forget(fuse_req_t req, fuse_ino_t ino, uint64_t nlookup) {
    fs_state_t *state = fuse_req_userdata(req);
    pthread_rwlock_wrlock(&state->lock);

    node_t *node = node_from_ino(req, ino);
    if (!node) {
        pthread_rwlock_unlock(&state->lock);

        return;
    }
    node->lookups -= nlookup;
    if (node->lookups <= 0) {
        ForgetNode(ino);
    }

    pthread_rwlock_unlock(&state->lock);
    fuse_reply_none(req);
}

static void s3_getattr(fuse_req_t req, fuse_ino_t ino, struct fuse_file_info *fi) {
    fs_state_t *state = fuse_req_userdata(req);
    pthread_rwlock_rdlock(&state->lock);

    node_t *node = node_from_ino(req, ino);
    if (!node) {
        pthread_rwlock_unlock(&state->lock);

        return;
    }

    struct stat stbuf = {0};
    fill_attr(fuse_req_ctx(req), &stbuf, node);
    int64_t offset = node->offset;
    pthread_rwlock_unlock(&state->lock);
    fuse_reply_attr(req, &stbuf, offset < 0 ? 0.0 : FUSE_TIMEOUT);
}

static void s3_open(fuse_req_t req, fuse_ino_t ino, struct fuse_file_info *fi) {
#if defined(__APPLE__)
    if (!IsValidOpen(fuse_req_ctx(req)->pid)) {
        fuse_reply_err(req, ECANCELED);

        return;
    }
#endif
    fs_state_t *state = fuse_req_userdata(req);
    pthread_rwlock_wrlock(&state->lock);

    node_t *node = node_from_ino(req, ino);
    if (!node) {
        pthread_rwlock_unlock(&state->lock);

        return;
    }
    if (S_ISDIR(node->stat.st_mode)) {
        pthread_rwlock_unlock(&state->lock);
        fuse_reply_err(req, EISDIR);

        return;
    }

    // At the moment the session only uses one thread so this should not cause a race condition
    if (node->offset == -1) {
        node->offset = CheckHeaderExistence(node);
    }
    fi->keep_cache = 1;
    pthread_rwlock_unlock(&state->lock);
    fuse_reply_open(req, fi);
}

static void s3_read(fuse_req_t req, fuse_ino_t ino, size_t size, off_t off,
                    struct fuse_file_info *fi) {
    fs_state_t *state = fuse_req_userdata(req);
    pthread_rwlock_rdlock(&state->lock);

    node_t *node = node_from_ino(req, ino);
    if (!node) {
        pthread_rwlock_unlock(&state->lock);

        return;
    }

    int err = 0;
    size_t n_bytes = 0;
    char *buf = DownloadData(node, size, off, &err, &n_bytes);
    pthread_rwlock_unlock(&state->lock);

    if (err != 0) {
        fuse_reply_err(req, err);
    } else {
        fuse_reply_buf(req, buf, n_bytes);
    }

    free(buf);
}

static void s3_release(fuse_req_t req, fuse_ino_t ino, struct fuse_file_info *fi) {
    fuse_reply_err(req, 0);
}

static void s3_opendir(fuse_req_t req, fuse_ino_t ino, struct fuse_file_info *fi) {
    fs_state_t *state = fuse_req_userdata(req);
    pthread_rwlock_rdlock(&state->lock);

    node_t *node = node_from_ino(req, ino);
    if (!node) {
        pthread_rwlock_unlock(&state->lock);

        return;
    }
    if (S_ISREG(node->stat.st_mode)) {
        pthread_rwlock_unlock(&state->lock);
        fuse_reply_err(req, ENOTDIR);

        return;
    }
    fi->keep_cache = 1;
    fi->cache_readdir = 1;
    pthread_rwlock_unlock(&state->lock);
    fuse_reply_open(req, fi);
}

static size_t dirbuf_add(fuse_req_t req, char *buf, size_t bufsize, const node_t *node,
                       const char *name, off_t off) {
    struct stat st = {0};
    fill_attr(fuse_req_ctx(req), &st, node);

    return fuse_add_direntry(req, buf, bufsize, name, &st, off + 1);
}

static void s3_readdir(fuse_req_t req, fuse_ino_t ino, size_t size, off_t off,
                       struct fuse_file_info *fi) {
    fs_state_t *state = fuse_req_userdata(req);
    pthread_rwlock_rdlock(&state->lock);

    node_t *node = node_from_ino(req, ino);
    if (!node) {
        pthread_rwlock_unlock(&state->lock);

        return;
    }

    char *buf = calloc(1, size);
    if (!buf) {
        pthread_rwlock_unlock(&state->lock);
        fuse_reply_err(req, ENOMEM);

        return;
    }

    off_t k = 0;
    size_t pos = 0; // Current position in the buffer

    // "."
    if (k >= off) {
        size_t next_size = dirbuf_add(req, buf + pos, size - pos, node, ".", k);
        if (pos + next_size > size) goto done;
        pos += next_size;
    }
    k++;

    // ".." if it exists
    if (node->parent) {
        if (k >= off) {
            size_t next_size = dirbuf_add(req, buf + pos, size - pos, node->parent, "..", k);
            if (pos + next_size > size) goto done;
            pos += next_size;
        }
        k++;
    }

    size_t i = (off - k > 0) ? off - k : 0;
    for (; i < node->chld_count; i++) {
        node_t *child = node->children[i];
        size_t next_size = dirbuf_add(req, buf + pos, size - pos, child, child->name, k + i);
        if (pos + next_size > size) goto done;
        pos += next_size;
    }

done:
    pthread_rwlock_unlock(&state->lock);
    fuse_reply_buf(req, buf, pos);
    free(buf);
}

static void s3_releasedir(fuse_req_t req, fuse_ino_t ino, struct fuse_file_info *fi) {
    fuse_reply_err(req, 0);
}

// All mutating operations return EROFS since the filesystem is read-only.
static void s3_setattr(fuse_req_t req, fuse_ino_t ino, struct stat *attr,
                       int to_set, struct fuse_file_info *fi) {
    fuse_reply_err(req, EROFS);
}

static void s3_write(fuse_req_t req, fuse_ino_t ino, const char *buf, size_t size,
                     off_t off, struct fuse_file_info *fi) {
    fuse_reply_err(req, EROFS);
}

static void s3_mkdir(fuse_req_t req, fuse_ino_t parent, const char *name, mode_t mode) {
    fuse_reply_err(req, EROFS);
}

static void s3_rmdir(fuse_req_t req, fuse_ino_t parent, const char *name) {
    fuse_reply_err(req, EROFS);
}

static void s3_unlink(fuse_req_t req, fuse_ino_t parent, const char *name) {
    fuse_reply_err(req, EROFS);
}

static void s3_rename(fuse_req_t req, fuse_ino_t parent, const char *name,
                      fuse_ino_t newparent, const char *newname, unsigned int flags) {
    fuse_reply_err(req, EROFS);
}

static void s3_create(fuse_req_t req, fuse_ino_t parent, const char *name,
                      mode_t mode, struct fuse_file_info *fi) {
    fuse_reply_err(req, EROFS);
}

static void s3_mknod(fuse_req_t req, fuse_ino_t parent, const char *name,
                     mode_t mode, dev_t rdev) {
    fuse_reply_err(req, EROFS);
}

static void s3_link(fuse_req_t req, fuse_ino_t ino, fuse_ino_t newparent,
                    const char *newname) {
    fuse_reply_err(req, EROFS);
}

static void s3_symlink(fuse_req_t req, const char *link, fuse_ino_t parent,
                       const char *name) {
    fuse_reply_err(req, EROFS);
}

static const struct fuse_lowlevel_ops operations = {
    .init       = s3_init,
    .destroy    = s3_destroy,
    .lookup     = s3_lookup,
    .forget     = s3_forget,
    .getattr    = s3_getattr,
    .open       = s3_open,
    .read       = s3_read,
    .release    = s3_release,
    .opendir    = s3_opendir,
    .readdir    = s3_readdir,
    .releasedir = s3_releasedir,
    .setattr    = s3_setattr,
    .write      = s3_write,
    .mkdir      = s3_mkdir,
    .rmdir      = s3_rmdir,
    .unlink     = s3_unlink,
    .rename     = s3_rename,
    .create     = s3_create,
    .mknod      = s3_mknod,
    .link       = s3_link,
    .symlink    = s3_symlink,
};

int mount_filesystem(fs_state_t *state, const char *mount, int debug) {
    pthread_rwlock_init(&state->lock, NULL);
    struct fuse_args args = FUSE_ARGS_INIT(0, NULL);

#if defined(__APPLE__)
    const char *basename = strrchr(mount, '/');
    if (!basename)
        basename = mount;
    else
        basename++; // skip leading slash

    int len = snprintf(NULL, 0, "defer_permissions,noapplexattr,noappledouble,iosize=%d,volname=%s", MAX_READ, basename);
    char *options = malloc(len + 1);
    if (options == NULL) {
        err(EXIT_FAILURE, "ERROR: Failed to allocate memory for options");
    }
    snprintf(options, len + 1, "defer_permissions,noapplexattr,noappledouble,iosize=%d,volname=%s", MAX_READ, basename);
#elif defined(__linux__)
    int len = snprintf(NULL, 0, "max_read=%d", MAX_READ);
    char *options = malloc(len + 1);
    if (options == NULL) {
        err(EXIT_FAILURE, "ERROR: Failed to allocate memory for options");
    }
    snprintf(options, len + 1, "max_read=%d", MAX_READ);
#endif

    int failure = fuse_opt_add_arg(&args, "data-gateway") ||
        fuse_opt_add_arg(&args, "-o") ||
        fuse_opt_add_arg(&args, options);

    free(options);

    if (debug)
        failure = failure || fuse_opt_add_arg(&args, "-d");

    if (failure) {
        fuse_opt_free_args(&args);

        errx(EXIT_FAILURE, "ERROR: Out of memory");
    }

    struct fuse_session *se = fuse_session_new(&args, &operations, sizeof(operations), state);
    fuse_opt_free_args(&args);
    if (!se)
        errx(EXIT_FAILURE, "ERROR: Session creation failed");

    if (fuse_set_signal_handlers(se) != 0) {
        fuse_session_destroy(se);

        errx(EXIT_FAILURE, "ERROR: Set signal handlers failed");
    }

    if (fuse_session_mount(se, mount) != 0) {
        fuse_remove_signal_handlers(se);
        fuse_session_destroy(se);

        errx(EXIT_FAILURE, "ERROR: Mounting failed");
    }

    // Single-threaded event loop
    state->se = se;
    int res = fuse_session_loop(se);

    fuse_session_unmount(se);
    fuse_remove_signal_handlers(se);
    fuse_session_destroy(se);
    pthread_rwlock_destroy(&state->lock);

    return res;
}

// invalidate_node is called when a node's cache needs to be invalidated
int invalidate_node(fs_state_t *state, node_t *node) {
    return fuse_lowlevel_notify_inval_inode(state->se, node->stat.st_ino, 0, 0);
}

// delete_node is called when kernel needs to be informed that the node no longer exists
int delete_node(fs_state_t *state, node_t *node) {
    return fuse_lowlevel_notify_delete(state->se, node->parent->stat.st_ino, node->stat.st_ino, node->name, strlen(node->name));
}

#endif
