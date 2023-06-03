// Copyright (C) 2023 - Perceval Faramaz
// SPDX-License-Identifier: GPL-2.0-only

#include "c_shim.h"

void pattern_step_pack(const pattern_step_padded_t* padded, void* packed_buf) {
	pattern_step_t *packed = (pattern_step_t*)packed_buf;
	packed->color = padded->color;
	packed->duration = padded->duration;
}
void pattern_step_pad(const pattern_step_t* packed, void* padded_buf) {
	pattern_step_padded_t *padded = (pattern_step_padded_t*)padded_buf;
	padded->color = packed->color;
	padded->duration = packed->duration;
}
int pattern_step_packed_size() {
	return sizeof(pattern_step_t);
}