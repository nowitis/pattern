// Copyright (C) 2023 - Perceval Faramaz
// SPDX-License-Identifier: GPL-2.0-only

#include "app_proto.h"
#include "blink.h"
#include <tk1_mem.h>
#include <types.h>

// clang-format off
static volatile uint32_t *led =   (volatile uint32_t *)TK1_MMIO_TK1_LED;

#define min(a,b) \
   ({ __typeof__ (a) _a = (a); \
       __typeof__ (b) _b = (b); \
     _a < _b ? _a : _b; })

#define abs(a) \
   (a < 0 ? -a : a)

#define LED_BLACK 0
#define LED_RED   (1 << TK1_MMIO_TK1_LED_R_BIT)
#define LED_GREEN (1 << TK1_MMIO_TK1_LED_G_BIT)
#define LED_BLUE  (1 << TK1_MMIO_TK1_LED_B_BIT)
#define LED_WHITE  (LED_RED | LED_BLUE | LED_GREEN)

// clang-format on

const uint8_t app_name0[4] = "tk1 ";
const uint8_t app_name1[4] = "ptrn";
const uint32_t app_version = 0x00000002;

void send_chunked(const uint8_t *buf, int buf_len, uint8_t *rsp, int cmd_len,
		  struct frame_header hdr, enum appcmd rspcode)
{
	const int chunk_len = cmd_len - 1;
	int nbytes = 0;
	for (int chunk_idx = 0; (chunk_idx * chunk_len + nbytes) < buf_len;
	     chunk_idx++) {
		nbytes = min(chunk_len, buf_len - (chunk_idx * chunk_len));

		rsp[0] = STATUS_OK;
		memcpy(&rsp[1], buf + (chunk_idx * chunk_len), nbytes);
		appreply(hdr, rspcode, rsp);
	}
}

int main(void)
{
	uint32_t stack;
	struct frame_header hdr; // Used in both directions
	uint8_t cmd[CMDLEN_MAXBYTES];
	uint8_t rsp[CMDLEN_MAXBYTES];

	int32_t nbytes_transferred = 0;
	uint8_t nsteps = 0;
	pattern_step_t pattern[128];
	uint8_t *pattern_buf = (uint8_t *)pattern;

	uint8_t in;

	qemu_puts("Hello! &stack is on: ");
	qemu_putinthex((uint32_t)&stack);
	qemu_lf();

	*led = LED_BLUE;

	for (;;) {
		in = readbyte();
		qemu_puts("Read byte: ");
		qemu_puthex(in);
		qemu_lf();

		if (parseframe(in, &hdr) == -1) {
			qemu_puts("Couldn't parse header\n");
			continue;
		}

		memset(cmd, 0, CMDLEN_MAXBYTES);
		// Read app command, blocking
		read(cmd, hdr.len);

		if (hdr.endpoint == DST_FW) {
			*led = LED_RED;
			appreply_nok(hdr);
			qemu_puts("Responded NOK to message meant for fw\n");
			continue;
		}

		// Is it for us?
		if (hdr.endpoint != DST_SW) {
			qemu_puts("Message not meant for app. endpoint was 0x");
			qemu_puthex(hdr.endpoint);
			qemu_lf();
			continue;
		}

		// Reset response buffer
		memset(rsp, 0, CMDLEN_MAXBYTES);

		if ((nbytes_transferred > 0) &&
		    (cmd[0] != APP_CMD_SET_PATTERN)) {
			appreply_nok(hdr);
			qemu_puts("Responded NOK as message was not expected "
				  "(expecting APP_CMD_SET_PATTERN)\n");
			continue;
		}

		if ((nbytes_transferred < 0) &&
		    (cmd[0] != APP_CMD_GET_PATTERN)) {
			appreply_nok(hdr);
			qemu_puts("Responded NOK as message was not expected "
				  "(expecting APP_CMD_GET_PATTERN)\n");
			continue;
		}

		// Min length is 1 byte so this should always be here
		switch (cmd[0]) {
		case APP_CMD_GET_NAMEVERSION:
			qemu_puts("APP_CMD_GET_NAMEVERSION\n");
			// only zeroes if unexpected cmdlen bytelen
			if (hdr.len == 1) {
				memcpy(rsp, app_name0, 4);
				memcpy(rsp + 4, app_name1, 4);
				memcpy(rsp + 8, &app_version, 4);
			}
			appreply(hdr, APP_RSP_GET_NAMEVERSION, rsp);
			break;

		case APP_CMD_SET_PATTERN: {
			qemu_puts("APP_CMD_SET_PATTERN\n");

			const int skipfirst = nbytes_transferred == 0;
			if (skipfirst) {
				nsteps = cmd[1];
			}

			if (nsteps > 128) {
				*led = LED_RED;
				rsp[0] = STATUS_BAD;
				appreply(hdr, APP_RSP_SET_PATTERN, rsp);
				break;
			}

			const int maxbytes = CMDLEN_MAXBYTES - 1 - skipfirst;
			const int nbytes =
			    min((sizeof(pattern_step_t) * nsteps) -
				    nbytes_transferred,
				maxbytes);
			memcpy(&pattern_buf[nbytes_transferred],
			       &cmd[1 + skipfirst], nbytes);

			nbytes_transferred += nbytes;

			if (nbytes_transferred ==
			    (nsteps * sizeof(pattern_step_t))) {
				*led = LED_GREEN;
				nbytes_transferred = 0;
			}

			rsp[0] = STATUS_OK;
			appreply(hdr, APP_RSP_SET_PATTERN, rsp);

			break;
		}

		case APP_CMD_EXECUTE:
			qemu_puts("APP_CMD_EXECUTE\n");

			pattern_execute(pattern, nsteps);

			rsp[0] = STATUS_OK;
			appreply(hdr, APP_RSP_EXECUTE, rsp);
			break;

		case APP_CMD_GET_PATTERN: {
			qemu_puts("APP_CMD_GET_PATTERN\n");

			// no pattern loaded
			if (nsteps == 0) {
				rsp[0] = STATUS_BAD;
				appreply(hdr, APP_RSP_GET_PATTERN, rsp);
				break;
			}

			const int isfirst = nbytes_transferred == 0;
			if (isfirst) {
				rsp[1] = nsteps;
			}

			const int maxbytes = CMDLEN_MAXBYTES - 1 - isfirst;
			const int nbytes =
			    min((sizeof(pattern_step_t) * nsteps) -
				    abs(nbytes_transferred),
				maxbytes);
			memcpy(&rsp[1 + isfirst],
			       &pattern_buf[abs(nbytes_transferred)], nbytes);

			nbytes_transferred -= nbytes;

			if (abs(nbytes_transferred) ==
			    (nsteps * sizeof(pattern_step_t))) {
				*led = LED_BLUE | LED_RED;
				nbytes_transferred = 0;
			}

			rsp[0] = STATUS_OK;
			appreply(hdr, APP_RSP_GET_PATTERN, rsp);

			break;
		}
		}
	}
}
