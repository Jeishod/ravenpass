#ifndef RAVENPASS_PASTEBOARD_H
#define RAVENPASS_PASTEBOARD_H

long ravenpass_pasteboard_write(const void *bytes, long length, const char *type);
long ravenpass_pasteboard_write_text(const void *bytes, long length);
long ravenpass_pasteboard_change_count(void);
void ravenpass_pasteboard_clear(void);

#endif
