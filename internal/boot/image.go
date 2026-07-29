package boot

import (
	"fmt"
	"os"
	"path/filepath"
)

// The loader lives at a different path depending on how the ISO was authored:
// case varies, and the ARM builds ship BOOTAA64.EFI where x86 ships BOOTX64.EFI.
// These lists are the layouts seen in practice, kept in probe order.
var (
	mbootPaths = []string{
		"EFI/BOOT/BOOTX64.EFI",
		"EFI/BOOT/BOOTAA64.EFI",
		"MBOOT.EFI",
		"mboot.efi",
		"efi/boot/bootx64.efi",
		"efi/boot/bootaa64.efi",
	}

	crypto64Paths = []string{
		"EFI/BOOT/CRYPTO64.EFI",
		"efi/boot/crypto64.efi",
	}

	// boot.cfg's own name varies between builds.
	configPaths = []string{
		"BOOT.CFG",
		"boot.cfg",
	}
)

// Mboot returns the path to the ESXi loader inside an extracted image.
func Mboot(imagePath string) (string, error) {
	return find(imagePath, mbootPaths, "mboot.efi")
}

// Crypto64 returns the path to the secure boot verifier inside an extracted
// image.
func Crypto64(imagePath string) (string, error) {
	return find(imagePath, crypto64Paths, "crypto64.efi")
}

// Config returns the path to the image's boot.cfg.
func Config(imagePath string) (string, error) {
	return find(imagePath, configPaths, "boot.cfg")
}

func find(imagePath string, candidates []string, what string) (string, error) {
	for _, c := range candidates {
		p := filepath.Join(imagePath, c)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}

	return "", fmt.Errorf("could not locate %s in %s", what, imagePath)
}
