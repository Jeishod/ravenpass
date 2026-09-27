#ifndef RAVENPASS_AUTOFILLBRIDGE_SYSTEM_H
#define RAVENPASS_AUTOFILLBRIDGE_SYSTEM_H

#include <stddef.h>

int ravenpass_self_satisfies(const char *requirement);
int ravenpass_self_team(char *team, size_t capacity);
int ravenpass_peer_satisfies(int socket, const char *requirement);
int ravenpass_group_container(const char *group, char *path, size_t capacity);
void ravenpass_post(const char *name);

#endif
