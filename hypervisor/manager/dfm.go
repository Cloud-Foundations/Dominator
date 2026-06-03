package manager

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"unicode"

	"github.com/Cloud-Foundations/Dominator/lib/types"
	proto "github.com/Cloud-Foundations/Dominator/proto/hypervisor"
)

// validateDfmProfile returns true if the profile contains only permitted
// characters.
func validateDfmProfile(profile string) bool {
	return !strings.ContainsFunc(profile, func(ch rune) bool {
		// Return true is character is not permitted.
		if unicode.IsDigit(ch) ||
			unicode.IsLetter(ch) {
			return false
		}
		switch ch {
		case '_':
			return false
		}
		return true
	})
}

func makeDfmVolume(filename string, id int, volume *proto.Volume) error {
	if !validateDfmProfile(string(volume.DFM.Profile)) {
		return fmt.Errorf("invalid DFM profile: \"%s\"", volume.DFM.Profile)
	}
	if volume.DFM.NvramSize > types.Bytes(volume.Size)>>1 {
		return fmt.Errorf("DFM NVRAM must be less than half volume size")
	}
	cmd := exec.Command("qemu-img", "create", "-f", "dfm",
		"-o",
		fmt.Sprintf("dfm_id=%d,profile=%s,wssd_file_bytes=%d,nvram_bytes=%d",
			id, volume.DFM.Profile, volume.Size-uint64(volume.DFM.NvramSize),
			volume.DFM.NvramSize),
		filename)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("error running %v: %s: %s",
			cmd.Args, err, string(output))
	}
	if fi, err := os.Stat(filename); err != nil {
		os.Remove(filename)
		return err
	} else {
		volume.Size = uint64(fi.Size())
	}
	return nil
}
