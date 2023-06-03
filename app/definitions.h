// Copyright (C) 2023 - Perceval Faramaz
// SPDX-License-Identifier: GPL-2.0-only

#ifdef PADDED
#define __packed
#define SUFFIXED_NAME(NAME) NAME##_padded_t

#else
#define __packed __attribute__((packed))
#define SUFFIXED_NAME(NAME) NAME##_t

#endif

typedef struct {
	uint8_t color;
	uint8_t duration;
} __packed SUFFIXED_NAME(pattern_step);