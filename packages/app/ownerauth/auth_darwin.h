#ifndef RAVENPASS_DEVICE_AUTH_H
#define RAVENPASS_DEVICE_AUTH_H

enum {
    RAVENPASS_AUTH_SUCCESS = 0,
    RAVENPASS_AUTH_CANCELED = 1,
    RAVENPASS_AUTH_FAILED = 2,
    RAVENPASS_AUTH_UNAVAILABLE = 3
};

typedef struct ravenpass_owner_prompt ravenpass_owner_prompt;

int ravenpass_device_owner_available(void);

// Returns NULL when allocation fails; every prompt it returns must be waited for and then freed.
ravenpass_owner_prompt *ravenpass_owner_prompt_start(const char *reason);
int ravenpass_owner_prompt_wait(ravenpass_owner_prompt *prompt);
void ravenpass_owner_prompt_cancel(ravenpass_owner_prompt *prompt);
void ravenpass_owner_prompt_free(ravenpass_owner_prompt *prompt);

#endif
