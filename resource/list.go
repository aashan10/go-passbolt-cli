package resource

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"al.essio.dev/pkg/shellescape"
	"github.com/passbolt/go-passbolt-cli/util"
	"github.com/passbolt/go-passbolt/api"
	"github.com/passbolt/go-passbolt/helper"
	"github.com/pterm/pterm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// DecryptedResource holds the result of decrypting a single resource.
type DecryptedResource struct {
	Index       int
	Resource    api.Resource
	Name        string
	Username    string
	URI         string
	Password    string
	Description string
	Err         error
}

// DecryptResourcesParallel decrypts resource metadata (and optionally secrets) in parallel.
// Exported for use by the TUI package.
func DecryptResourcesParallel(ctx context.Context, client *api.Client, resources []api.Resource, needSecrets bool) ([]DecryptedResource, error) {
	return decryptResourcesParallel(ctx, client, resources, needSecrets)
}

var defaultTableColumns = []string{"ID", "FolderParentID", "Name", "Username", "URI"}

// ResourceListCmd Lists a Passbolt Resource
var ResourceListCmd = &cobra.Command{
	Use:     "resource",
	Short:   "Lists Passbolt Resources",
	Long:    `Lists Passbolt Resources`,
	Aliases: []string{"resources"},
	RunE:    ResourceList,
}

func init() {
	flags := ResourceListCmd.Flags()
	flags.Bool("favorite", false, "Resources that are marked as favorite")
	flags.Bool("own", false, "Resources that are owned by me")
	flags.StringP("group", "g", "", "Resources that are shared with group")
	flags.StringArrayP("folder", "f", []string{}, "Resources that are in folder")
	flags.StringArrayP("column", "c", defaultTableColumns, "Columns to return (default list only for table format; JSON format includes all fields by default).\nPossible Columns: ID, FolderParentID, Name, Username, URI, Password, Description, CreatedTimestamp, ModifiedTimestamp")
}

type resourceListConfig struct {
	favorite       bool
	own            bool
	group          string
	folderParents  []string
	columns        []string
	columnsChanged bool
	jsonOutput     bool
	celFilter      string
}

func ResourceList(cmd *cobra.Command, args []string) error {
	config, err := parseResourceListFlags(cmd)
	if err != nil {
		return err
	}

	// Check if we need to fetch secrets (expensive server join + RSA decryption)
	// For v5 resources, metadata (name, username, uri) can be decrypted without secrets
	needSecrets := false
	for _, col := range config.columns {
		switch strings.ToLower(col) {
		case "password", "description":
			needSecrets = true
		}
	}

	// Check if CEL filter references Password or Description
	if !needSecrets && config.celFilter != "" {
		refsSecrets, err := util.CELExpressionReferencesFields(config.celFilter, []string{"Password", "Description"}, CelEnvOptions...)
		if err != nil {
			return fmt.Errorf("Parsing filter: %w", err)
		}
		needSecrets = refsSecrets
	}

	ctx, cancel := util.GetContext()
	defer cancel()

	client, err := util.GetClient(ctx)
	if err != nil {
		return err
	}
	defer util.SaveSessionKeysAndLogout(ctx, client)
	cmd.SilenceUsage = true

	resources, err := client.GetResources(ctx, &api.GetResourcesOptions{
		FilterIsFavorite:        config.favorite,
		FilterIsOwnedByMe:       config.own,
		FilterIsSharedWithGroup: config.group,
		FilterHasParent:         config.folderParents,
		ContainSecret:           needSecrets,
	})
	if err != nil {
		return fmt.Errorf("Listing Resource: %w", err)
	}

	// Decrypt all resources in parallel
	decrypted, err := decryptResourcesParallel(ctx, client, resources, needSecrets)
	if err != nil {
		return err
	}

	// Apply CEL filter on already-decrypted data
	if config.celFilter != "" {
		decrypted, err = filterDecryptedResources(decrypted, config.celFilter, ctx)
		if err != nil {
			return err
		}
	}

	if config.jsonOutput {
		return printJsonResources(decrypted, config.columnsChanged, config.columns)
	}

	return printTableResources(decrypted, config.columns)
}

func decryptResourcesParallel(ctx context.Context, client *api.Client, resources []api.Resource, needSecrets bool) ([]DecryptedResource, error) {
	// Use parallel decryption with worker pool
	numWorkers := int(viper.GetUint("workers"))

	// Limit Worker count to Resource count
	if len(resources) < numWorkers {
		numWorkers = len(resources)
	}

	// Filter resources - only require secrets if we're fetching them
	var validResources []api.Resource
	for i := range resources {
		if needSecrets && len(resources[i].Secrets) == 0 {
			continue
		}
		validResources = append(validResources, resources[i])
	}

	if len(validResources) == 0 {
		return []DecryptedResource{}, nil
	}

	// Channel for work items and results
	// Note: Session keys are pre-fetched during Login() when the server supports v5 metadata,
	// so no additional prefetching is needed here.
	jobs := make(chan int, len(validResources))
	results := make(chan DecryptedResource, len(validResources))

	// Start workers
	var wg sync.WaitGroup
	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				resource := validResources[idx]

				// Lookup resource type from cache (single API call for all types)
				rType, err := client.GetResourceTypeCached(ctx, resource.ResourceTypeID)
				if err != nil {
					results <- DecryptedResource{Index: idx, Err: fmt.Errorf("Get ResourceType: %w", err)}
					continue
				}

				// For v4 resources without secret decryption, use plaintext fields directly
				// This avoids unnecessary function calls for 10k+ resources
				isV5 := strings.HasPrefix(rType.Slug, "v5-")
				if !needSecrets && !isV5 {
					// V4 resource - metadata is plaintext, no decryption needed
					results <- DecryptedResource{
						Index:       idx,
						Resource:    resource,
						Name:        resource.Name,
						Username:    resource.Username,
						URI:         resource.URI,
						Password:    "",
						Description: resource.Description,
					}
					continue
				}

				// Handle case where secrets weren't fetched
				var secret api.Secret
				if len(resource.Secrets) > 0 {
					secret = resource.Secrets[0]
				}

				_, name, username, uri, pass, desc, err := helper.GetResourceFromDataWithOptions(
					client,
					resource,
					secret,
					*rType,
					needSecrets,
				)
				results <- DecryptedResource{
					Index:       idx,
					Resource:    resource,
					Name:        name,
					Username:    username,
					URI:         uri,
					Password:    pass,
					Description: desc,
					Err:         err,
				}
			}
		}()
	}

	// Send jobs
	for i := range validResources {
		jobs <- i
	}
	close(jobs)

	// Wait for workers and close results
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect all results first
	allResults := make([]DecryptedResource, len(validResources))
	for result := range results {
		allResults[result.Index] = result
	}

	// Process results, skipping unsupported types
	decrypted := make([]DecryptedResource, 0, len(validResources))
	skippedTypes := make(map[string]int)

	for _, result := range allResults {
		if result.Err != nil {
			if errors.Is(result.Err, helper.ErrUnsupportedResourceType) {
				// Get type slug for warning message
				rType, _ := client.GetResourceTypeCached(ctx, result.Resource.ResourceTypeID)
				typeSlug := "unknown"
				if rType != nil {
					typeSlug = rType.Slug
				}
				skippedTypes[typeSlug]++
				continue
			}
			// Other errors are still fatal
			return nil, fmt.Errorf("Get Resource %w", result.Err)
		}
		decrypted = append(decrypted, result)
	}

	// Print warning summary to stderr
	if len(skippedTypes) > 0 {
		total := 0
		for _, count := range skippedTypes {
			total += count
		}
		fmt.Fprintf(os.Stderr, "Warning: %d resource(s) skipped due to unsupported types:\n", total)
		for typeSlug, count := range skippedTypes {
			fmt.Fprintf(os.Stderr, "  - %s: %d\n", typeSlug, count)
		}
	}

	return decrypted, nil
}

func printJsonResources(
	decrypted []DecryptedResource,
	isColumnsChanged bool,
	columns []string,
) error {
	outputResources := make([]ResourceJsonOutput, len(decrypted))
	for i, d := range decrypted {
		name := d.Name
		username := d.Username
		uri := d.URI
		pass := d.Password
		desc := d.Description
		outputResources[i] = ResourceJsonOutput{
			ID:                &d.Resource.ID,
			FolderParentID:    &d.Resource.FolderParentID,
			Name:              &name,
			Username:          &username,
			URI:               &uri,
			Password:          &pass,
			Description:       &desc,
			CreatedTimestamp:  &d.Resource.Created.Time,
			ModifiedTimestamp: &d.Resource.Modified.Time,
		}
	}

	if isColumnsChanged {
		filteredMap := make([]map[string]interface{}, len(outputResources))
		for i := range outputResources {
			filteredMap[i] = make(map[string]interface{})
			data, _ := json.Marshal(outputResources[i])
			var resourceMap map[string]interface{}
			json.Unmarshal(data, &resourceMap)

			for _, col := range columns {
				col = strings.ToLower(col)

				if val, ok := resourceMap[col]; ok {
					filteredMap[i][col] = val
				}
			}
		}

		jsonResources, err := json.MarshalIndent(filteredMap, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(jsonResources))
		return nil
	}

	jsonResources, err := json.MarshalIndent(outputResources, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(jsonResources))
	return nil
}

func printTableResources(
	decrypted []DecryptedResource,
	columns []string,
) error {
	data := pterm.TableData{columns}

	for _, d := range decrypted {
		entry := make([]string, len(columns))
		for i := range columns {
			switch strings.ToLower(columns[i]) {
			case "id":
				entry[i] = d.Resource.ID
			case "folderparentid":
				entry[i] = d.Resource.FolderParentID
			case "name":
				entry[i] = shellescape.StripUnsafe(d.Name)
			case "username":
				entry[i] = shellescape.StripUnsafe(d.Username)
			case "uri":
				entry[i] = shellescape.StripUnsafe(d.URI)
			case "password":
				entry[i] = shellescape.StripUnsafe(d.Password)
			case "description":
				entry[i] = shellescape.StripUnsafe(d.Description)
			case "createdtimestamp":
				entry[i] = d.Resource.Created.Format(time.RFC3339)
			case "modifiedtimestamp":
				entry[i] = d.Resource.Modified.Format(time.RFC3339)
			default:
				return fmt.Errorf("Unknown Column: %v", columns[i])
			}
		}
		data = append(data, entry)
	}

	pterm.DefaultTable.WithHasHeader().WithData(data).Render()
	return nil
}

func parseResourceListFlags(cmd *cobra.Command) (*resourceListConfig, error) {
	favorite, err := cmd.Flags().GetBool("favorite")
	if err != nil {
		return nil, err
	}
	own, err := cmd.Flags().GetBool("own")
	if err != nil {
		return nil, err
	}
	group, err := cmd.Flags().GetString("group")
	if err != nil {
		return nil, err
	}
	folderParents, err := cmd.Flags().GetStringArray("folder")
	if err != nil {
		return nil, err
	}
	columns, err := cmd.Flags().GetStringArray("column")
	if err != nil {
		return nil, err
	}
	if len(columns) == 0 {
		return nil, fmt.Errorf("You need to specify at least one column to return")
	}
	jsonOutput, err := cmd.Flags().GetBool("json")
	if err != nil {
		return nil, err
	}
	celFilter, err := cmd.Flags().GetString("filter")
	if err != nil {
		return nil, err
	}

	return &resourceListConfig{
		favorite:       favorite,
		own:            own,
		group:          group,
		folderParents:  folderParents,
		columns:        columns,
		columnsChanged: cmd.Flags().Changed("column"),
		jsonOutput:     jsonOutput,
		celFilter:      celFilter,
	}, nil
}
