// Package fonts embeds the Inter and Chakra Petch typefaces and registers them with fontconfig
// at application startup so the drawer renders identically across desktop
// (KDE/Wayland) and gamescope (Steam Gaming Mode) environments.
package fonts

import (
	_ "embed"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"syscall"
	"unsafe" //nolint:gocritic // used with cgo, requires separate import block
)

// #cgo pkg-config: fontconfig
// #include <fontconfig/fontconfig.h>
// #include <stdlib.h>
import "C" //nolint:gocritic // cgo requires standalone import

//go:embed Inter-Regular.ttf
var interRegular []byte

//go:embed Inter-Bold.ttf
var interBold []byte

// Chakra Petch SemiBold (SIL OFL, see OFL-ChakraPetch.txt) is the display face for
// buttons, tabs and labels: angular and wide, readable at 13px.
//
//go:embed ChakraPetch-SemiBold.ttf
var chakraPetchSemiBold []byte

// Register writes the embedded Inter font files to a runtime directory and
// registers them with fontconfig via FcConfigAppFontAddFile. This makes the
// "Inter" family available to GTK/Pango CSS without installing system-wide.
// Must be called before any GTK CSS providers are loaded.
func Register() {
	base := filepath.Join(runtimeDir(), "z13gui-plus")
	dir := filepath.Join(base, "fonts")
	if err := os.MkdirAll(dir, 0o700); err != nil { //nolint:gocritic // err is checked
		slog.Warn("fonts: cannot create dir", "path", dir, "err", err)
		return
	}
	// The /tmp fallback is shared: another user could pre-create these
	// directories with symlinks inside to make us overwrite their targets.
	for _, d := range []string{base, dir} {
		if err := checkPrivateDir(d); err != nil { //nolint:gocritic // err is checked
			slog.Warn("fonts: refusing unsafe dir", "path", d, "err", err)
			return
		}
	}

	for _, f := range []struct {
		name string
		data []byte
	}{
		{"Inter-Regular.ttf", interRegular},
		{"Inter-Bold.ttf", interBold},
		{"ChakraPetch-SemiBold.ttf", chakraPetchSemiBold},
	} {
		path := filepath.Join(dir, f.name)
		// Skip write if file already exists with correct size.
		if info, err := os.Stat(path); err != nil || info.Size() != int64(len(f.data)) {
			if err := os.WriteFile(path, f.data, 0o644); err != nil { //nolint:gocritic // err is checked
				slog.Warn("fonts: write failed", "path", path, "err", err)
				continue
			}
		}
		cpath := C.CString(path)
		ok := C.FcConfigAppFontAddFile(C.FcConfigGetCurrent(), (*C.FcChar8)(unsafe.Pointer(cpath)))
		C.free(unsafe.Pointer(cpath))
		if ok == C.FcTrue {
			slog.Debug("fonts: registered", "path", path)
		} else {
			slog.Warn("fonts: FcConfigAppFontAddFile failed", "path", path)
		}
	}
}

// checkPrivateDir reports an error unless path is a real directory (not a
// symlink) owned by the current user and inaccessible to anyone else.
func checkPrivateDir(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("not a directory")
	}
	if st, ok := info.Sys().(*syscall.Stat_t); !ok || int(st.Uid) != os.Getuid() {
		return fmt.Errorf("not owned by current user")
	}
	if info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("mode %v is accessible to other users", info.Mode().Perm())
	}
	return nil
}

// runtimeDir returns XDG_RUNTIME_DIR or falls back to /tmp.
func runtimeDir() string {
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		return d
	}
	return "/tmp"
}
