#include "textflag.h"

// func cpu() uint64
TEXT ·cpu(SB),NOSPLIT,$0-8
	MOVL	$0x0b, AX // version information
	MOVL	$0x00, BX // any leaf will do
	MOVL	$0x00, CX // any subleaf will do

	// call CPUID
	BYTE $0x0f
	BYTE $0xa2
	MOVQ	DX, ret+0(FP) // logical cpu id is put in EDX
	RET