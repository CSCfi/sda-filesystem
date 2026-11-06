#ifndef HELPERS_H
#define HELPERS_H

#include <sys/stat.h>
#include <stdlib.h>
#include <stdint.h>
#include <time.h>
#include <pthread.h>

// Node represents one file or directory
typedef struct Node {
	char *name;
	char *orig_name;
	struct Node *parent;
	struct Node **children;
	size_t chld_count;
	struct stat stat;
	struct timespec last_modified;
	// Offset is initialized as -1 indicating the header has not yet been fetched.
	// Value -2 indicates the object was removed from storage and should not be shown in the filesystem.
	// A non-negative value represents an actual offset in the object.
	int64_t offset;
	int64_t lookups;
} node_t;

// FsState holds the filesystem root and the rwlock that guards it.
// Passed as FUSE userdata so callbacks never need a global.
typedef struct FsState {
	node_t          *root;
	pthread_rwlock_t lock;
	struct fuse_session *se;
} fs_state_t;

node_t *search_node(const node_t *root, const char *path);
node_t *find_child_node(const node_t *parent, const char *name);
void sort_node_children(const node_t *node);
void free_nodes(node_t *node);

#endif
