#ifndef RAVENPASS_IDENTITYSTORE_H
#define RAVENPASS_IDENTITYSTORE_H

#include <stddef.h>

int ravenpass_identities_enabled(void);
int ravenpass_identities_replace(const char *listed, size_t length);
int ravenpass_identities_remove_all(void);

#endif
