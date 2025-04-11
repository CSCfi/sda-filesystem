#if defined(__linux__)
#define FUSE_USE_VERSION 316
#else
#define FUSE_USE_VERSION 29
#endif

#include <fuse.h>
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

const int MAX_READ = 1 << 20;

static void s3_destroy(void *private_data) {
    WaitForLock();

    nodes_t *n = (nodes_t *)private_data;
    free_nodes(n);
    free(n);
}

static int s3_open(const char *path, struct fuse_file_info *fi) {
    struct fuse_context *fc = fuse_get_context();
    if (!IsValidOpen(fc->pid))
        return -ECANCELED;

    node_t *node = search_node((nodes_t *)fc->private_data, path);
	if (!node) {
		return -ENOENT;
	}
	if (S_ISDIR(node->stat.st_mode)) {
		return -EISDIR;
    }
    fi->fh = node->stat.st_ino; // This will be reflected in read()

    if (node->offset == -1) {
        node->offset = 0;
        CheckHeaderExistence(node, path);
    }

	return 0;
}

static int s3_read(const char *path, char *buf, size_t size, off_t off, struct fuse_file_info *fi) {
    struct fuse_context *fc = fuse_get_context();
    node_t *node = ((nodes_t *)fc->private_data)->nodes + fi->fh;

    int n_bytes = DownloadData(node, path, buf, size, off);
    if (n_bytes == -1) {
        return -EFAULT;
    } else if (n_bytes == -2) {
        return -EACCES;
    } else if (n_bytes < -2){
        return -EIO;
    }

    return n_bytes;
}

static int s3_opendir(const char *path, struct fuse_file_info *fi) {
    struct fuse_context *fc = fuse_get_context();
    node_t *node = search_node((nodes_t *)fc->private_data, path);
	if (!node) {
		return -ENOENT;
	}
	if (S_ISREG(node->stat.st_mode)) {
		return -ENOTDIR;
	}
    fi->fh = node->stat.st_ino; // This will be reflected in readdir()

	return 0;
}

static int s3_readdir(const char *path, void *buf, fuse_fill_dir_t filler,
                      off_t offset, struct fuse_file_info *fi) {
    struct fuse_context *fc = fuse_get_context();
    nodes_t *n = (nodes_t *)fc->private_data;
    node_t *node = n->nodes + fi->fh;

#if defined(__linux__)
    filler(buf, ".", &node->stat, 0, 0);
	filler(buf, "..", NULL, 0, 0);
#elif defined(__APPLE__)
    filler(buf, ".", &node->stat, 0);
	filler(buf, "..", NULL, 0);
#endif

    for (int64_t i = 0; i < node->chld_count; i++) {
        node_t *chld = node->children + i;
        if (S_ISDIR(chld->stat.st_mode) && !chld->chld_count)
            continue;

#if defined(__linux__)
        if (filler(buf, chld->name, &chld->stat, 0, 0))
            break;
#elif defined(__APPLE__)
        if (filler(buf, chld->name, &chld->stat, 0))
            break;
#endif
    }

    return 0;
}

#if defined(__linux__)
static int s3_readdir3(const char *path, void *buf, fuse_fill_dir_t filler,
                       off_t offset, struct fuse_file_info *fi, enum fuse_readdir_flags flags) {
    return s3_readdir(path, buf, filler, offset, fi);
}
#endif

static int s3_getattr(const char *path, struct stat *stbuf) {
    struct fuse_context *fc = fuse_get_context();
    nodes_t *n = (nodes_t *)fc->private_data;
    node_t *node = search_node(n, path);
	if (!node) {
		return -ENOENT;
	}

    *stbuf = node->stat;
#if defined(__linux__)
    stbuf->st_atim = node->last_modified;
    stbuf->st_ctim = node->last_modified;
    stbuf->st_mtim = node->last_modified;
#elif defined(__APPLE__)
    stbuf->st_birthtimespec = node->last_modified;
    stbuf->st_atimespec = node->last_modified;
    stbuf->st_ctimespec = node->last_modified;
    stbuf->st_mtimespec = node->last_modified;
#endif
    stbuf->st_uid = n->uid;
    stbuf->st_gid = n->gid;

    return 0;
}

static void *s3_init(struct fuse_conn_info *conn) {
    nodes_t *n = GetFilesystem();
    n->uid = getuid();
    n->gid = getgid();

    return n;
}

#if defined(__linux__)
static int s3_getattr3(const char *path, struct stat *stbuf, struct fuse_file_info *fi) {
    return s3_getattr(path, stbuf);
}

static void *s3_init3(struct fuse_conn_info *conn, struct fuse_config *cfg) {
    conn->max_read = MAX_READ;
    conn->max_readahead = MAX_READ;
    (void) cfg;

    return s3_init(conn);
}
#endif

static const struct fuse_operations operations = {
    .destroy	= s3_destroy,
	.open		= s3_open,
	.read		= s3_read,
    .opendir	= s3_opendir,
#if defined(__linux__)
    .readdir	= s3_readdir3,
    .getattr	= s3_getattr3,
    .init       = s3_init3,
#elif defined(__APPLE__)
	.readdir	= s3_readdir,
    .getattr	= s3_getattr,
    .init       = s3_init,
#endif
};

int mount_filesystem(const char *mount, int debug) {
    struct fuse_args args = FUSE_ARGS_INIT(0, NULL);
    char *options = strdup("auto_cache");

#if defined(__APPLE__)
    const char *basename = strrchr(mount, '/');
    if (!basename)
        basename = mount;
    else // remove leading slash
        basename++;

    char macos_options[100];
    sprintf(macos_options, ",defer_permissions,noapplexattr,noappledouble,iosize=%d,volname=", MAX_READ);
    options = (char*)realloc(options, strlen(options) + strlen(macos_options) + strlen(basename) + 1);
    strcat(options, macos_options);
    strcat(options, basename);
#elif defined(__linux__)
    char linux_options[20];
    sprintf(linux_options, ",max_read=%d", MAX_READ);
    options = (char*)realloc(options, strlen(options) + strlen(linux_options) + 1);
    strcat(options, linux_options);
#endif

    if (debug) {
        options = (char*)realloc(options, strlen(options) + 7);
        strcat(options, ",debug");
    }

    int failure = fuse_opt_add_arg(&args, "data-gateway") ||
        fuse_opt_add_arg(&args, mount) ||
        fuse_opt_add_arg(&args, "-f") || // foreground
        fuse_opt_add_arg(&args, "-s") || // single thread
        fuse_opt_add_arg(&args, "-o") ||
        fuse_opt_add_arg(&args, options);
    free(options);

    if (failure) {
        errx(3, "ERROR: Out of memory");
    }

    umask(0222);
    int res = fuse_main(args.argc, args.argv, &operations, NULL);
    fuse_opt_free_args(&args);

    return res;
}
