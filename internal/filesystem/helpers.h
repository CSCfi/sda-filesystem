#ifndef HELPERS_H
#define HELPERS_H

#include <sys/stat.h>
#include <stdlib.h>
#include <stdint.h>
#include <time.h>

typedef const char cchar_t;

// Node represents one file or directory
typedef struct Node {
	char *name;
	char *orig_name;
	struct Node *parent;
	struct Node *children;
	int64_t chld_count;
	struct stat stat;
	struct timespec last_modified;
	// Offset is initialized as -1 indicating the header has not yet been fetched.
	// Value -2 indicates the object was removed from storage and should not be shown in the filesystem.
	// A non-negative value represents an actual offset in the object.
	int64_t offset;
} node_t;

typedef struct Nodes {
	struct Node *nodes;
	int64_t count;
	uid_t uid;
	gid_t gid;
} nodes_t;

node_t *search_node(nodes_t *n, const char *path);
void sort_node_children(node_t *node);
void free_nodes(nodes_t *n);

#endif
