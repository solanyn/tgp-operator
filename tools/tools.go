//go:build tools

package tools

import (
	_ "sigs.k8s.io/controller-tools/cmd/controller-gen"
	_ "github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen"
	_ "github.com/Khan/genqlient"
)