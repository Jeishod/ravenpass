#ifndef RAVENPASS_APPEARANCE_H
#define RAVENPASS_APPEARANCE_H

typedef enum {
    ravenpass_appearance_system,
    ravenpass_appearance_light,
    ravenpass_appearance_dark,
} ravenpass_appearance;

void ravenpass_appearance_set(ravenpass_appearance appearance);

#endif
