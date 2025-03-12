#include <string.h>
#include <stdio.h>
#include "helpers.h"

static int compare_nodes(const void *n1, const void *n2) {
    return strcmp(((node_t *)n1)->name, ((node_t *)n2)->name);
}

// search_node finds the node at the end of path
node_t *search_node(nodes_t *n, const char *path) {
	node_t *node = n->nodes;
    char *path_copy = strdup(path);

    for (char *p = strtok(path_copy, "/"); p != NULL; p = strtok(NULL, "/")) {
        if (!*p) // empty string
            continue;

        node_t node_find;
        node_find.name = p;
        node = bsearch(&node_find, node->children, node->chld_count, sizeof(node_t), compare_nodes);
        if (!node)
            break;
    }

    free(path_copy);

	return node;
}

void sort_node_children(node_t *node) {
	qsort(node->children, node->chld_count, sizeof(node_t), compare_nodes);
}

// free_node frees the dynamically allocated fields of `node` and all the fields of its descendants
static void free_node(node_t *node) {
    free(node->name);
    free(node->orig_name);

    for (int i = 0; i < node->chld_count; i++) {
        free_node(node->children + i);
    }
}

void free_nodes(nodes_t *n) {
    free_node(n->nodes);
    free(n->nodes);
}
