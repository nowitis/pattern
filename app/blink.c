// Copyright (C) 2023 - Perceval Faramaz
// SPDX-License-Identifier: GPL-2.0-only

// clang-format off
#include <tk1_mem.h>
#include <types.h>
#include "blink.h"

static volatile uint32_t *led = (volatile uint32_t *)TK1_MMIO_TK1_LED;
// clang-format on

#define LED_BLACK 0
#define DOT_DURATION 300000

void pattern_execute(pattern_step_t const *pattern, int count)
{
	*led = 0;
	for (int i = 0; i < count; i++, pattern++) {
		pattern_step_t step = *pattern;

		*led = step.color;
		for (int i = 0; i < (step.duration * DOT_DURATION); i++) {
			__asm("");
		}
	}
}