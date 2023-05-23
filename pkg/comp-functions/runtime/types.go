package runtime

import (
	"context"
)

// AppInfo defines application information
type AppInfo struct {
	Version, Commit, Date, AppName, AppLongName string
}

// Transform specifies a transformation function to be run against the given FunctionIO.
type Transform struct {
	Name          string
	TransformFunc func(c context.Context, io *Runtime) Result
}
