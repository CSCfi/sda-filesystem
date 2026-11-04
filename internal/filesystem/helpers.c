#include <string.h>
#include <stdio.h>
#include "helpers.h"

static int compare_nodes(const void *n1, const void *n2) {
    return strcmp((*(node_t **)n1)->name, (*(node_t **)n2)->name);
}

// search_node finds the node at the end of path
node_t *search_node(const node_t *root, const char *path) {
	const node_t *node = root;
    char *path_copy = strdup(path);

    char *save;
    for (char *p = strtok_r(path_copy, "/", &save); p != NULL; p = strtok_r(NULL, "/", &save)) {
        if (!*p) // empty string
            continue;

        node = find_child_node(node, p);
        if (!node)
            break;
    }

    free(path_copy);

	return (node_t *)node;
}

node_t *find_child_node(const node_t *parent, const char *name) {
    node_t key;
    key.name = (char *)name;
    node_t *key_ptr = &key;
    node_t **found = bsearch(&key_ptr, parent->children, parent->chld_count, sizeof(node_t*), compare_nodes);

    return found ? *found : NULL;
}

void sort_node_children(const node_t *node) {
	qsort(node->children, node->chld_count, sizeof(node_t*), compare_nodes);
}

// free_nodes frees all the fields in `node` and recursively goes
// through all the children and frees their memory as well.
// The function does not free the memory of the node itself.
void free_nodes(node_t *node) {
    int64_t old_count = node->chld_count;
    node->chld_count = 0;
    for (int i = 0; i < old_count; i++) {
        free_nodes(node->children[i]);
        free(node->children[i]);
    }

    free(node->children);
    free(node->name);
    free(node->orig_name);
    node->children = NULL;
    node->name = NULL;
    node->orig_name = NULL;
}
