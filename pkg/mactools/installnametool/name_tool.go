package installnametool

import (
	"context"

	"github.com/ironpark/zapp/pkg/mactools/internal/macexec"
)

// Change rewrites a dependent shared library install name (LC_LOAD_DYLIB).
func Change(ctx context.Context, old string, new string, file string) error {
	return run(ctx, "-change", old, new, file)
}

// ChangeId sets the install name of a shared library (LC_ID_DYLIB).
func ChangeId(ctx context.Context, new string, file string) error {
	return run(ctx, "-id", new, file)
}

// AddRPath appends a runpath (LC_RPATH) entry.
func AddRPath(ctx context.Context, rpath string, file string) error {
	return run(ctx, "-add_rpath", rpath, file)
}

func run(ctx context.Context, args ...string) error {
	_, err := macexec.Run(ctx, "install_name_tool", args...)
	return err
}
