// Copyright (C) 2023 - Perceval Faramaz
// SPDX-License-Identifier: GPL-2.0-only

#include <stdint.h>

#define PADDED
#include "definitions.h"

#undef PADDED
#undef __packed
#undef SUFFIXED_NAME
#include "definitions.h"

int pattern_step_packed_size();
void pattern_step_pack(const pattern_step_padded_t* padded, void* packed_buf);
void pattern_step_pad(const pattern_step_t* packed, void* padded_buf);