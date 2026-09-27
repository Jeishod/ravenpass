#ifndef RAVENPASS_PRINTING_H
#define RAVENPASS_PRINTING_H

#include <stddef.h>

// Shows the print panel for a UTF-8 HTML page and blocks until it closes; returns 0 once the owner prints or cancels,
// -1 where the panel cannot be shown. The caller must not be the main thread.
int ravenpass_print_page(const char *job, size_t jobLength, const char *page, size_t pageLength);

#endif
