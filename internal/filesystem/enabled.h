#ifndef ENABLED_H
#define ENABLED_H

int mount_filesystem(fs_state_t *state, const char *mount, int debug);
int invalidate_node(fs_state_t *state, node_t *node);
int delete_node(fs_state_t *state, node_t *node);

#endif
