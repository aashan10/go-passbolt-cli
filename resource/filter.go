package resource

import (
	"context"
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/passbolt/go-passbolt-cli/util"
)

// CelEnvOptions defines the CEL environment for resource filtering
var CelEnvOptions = []cel.EnvOption{
	cel.Variable("ID", cel.StringType),
	cel.Variable("FolderParentID", cel.StringType),
	cel.Variable("Name", cel.StringType),
	cel.Variable("Username", cel.StringType),
	cel.Variable("URI", cel.StringType),
	cel.Variable("Password", cel.StringType),
	cel.Variable("Description", cel.StringType),
	cel.Variable("CreatedTimestamp", cel.TimestampType),
	cel.Variable("ModifiedTimestamp", cel.TimestampType),
}

// filterDecryptedResources filters already-decrypted resources by evaluating a CEL expression.
func filterDecryptedResources(resources []DecryptedResource, celCmd string, ctx context.Context) ([]DecryptedResource, error) {
	if celCmd == "" {
		return resources, nil
	}

	program, err := util.InitCELProgram(celCmd, CelEnvOptions...)
	if err != nil {
		return nil, err
	}

	filtered := []DecryptedResource{}
	for _, d := range resources {
		val, _, err := (*program).ContextEval(ctx, map[string]any{
			"ID":                d.Resource.ID,
			"FolderParentID":    d.Resource.FolderParentID,
			"Name":              d.Name,
			"Username":          d.Username,
			"URI":               d.URI,
			"Password":          d.Password,
			"Description":       d.Description,
			"CreatedTimestamp":  d.Resource.Created.Time,
			"ModifiedTimestamp": d.Resource.Modified.Time,
		})

		if err != nil {
			return nil, err
		}

		if val.Value() == true {
			filtered = append(filtered, d)
		}
	}

	if len(filtered) == 0 {
		return nil, fmt.Errorf("No such Resources found with filter %v!", celCmd)
	}
	return filtered, nil
}
