package inertiaproto

import (
	"errors"
	"fmt"

	"buf.build/go/protovalidate"
	"github.com/go-json-experiment/json"
	"go.inout.gg/foundations/http/httpmiddleware"
	"go.inout.gg/foundations/must"
	"go.segfaultmedaddy.com/inertia"
	"go.segfaultmedaddy.com/inertia/contrib/vite"
	"go.segfaultmedaddy.com/inertia/inertiaframe"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type Endpoint[M proto.Message] = inertiaframe.Endpoint[M]

const RootTemplate = `<!doctype html>
<html>
  <head>
    <meta charset="utf-8" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    {{.InertiaHead}}
  </head>
  <body>
    {{.InertiaBody}}
    {{template "viteClient"}}
    {{template "viteReactRefresh"}}
    {{viteResource "src/main.tsx"}}
  </body>
</html>
`

type Config struct {
	BundleVersion string
	ViteConfig    *vite.Config
	SSRClient     inertia.SSRClient
}

// NewMiddleware creates a new middleware that handles inertia requests
// using protobuf messages.
func NewMiddleware(opts ...func(*Config)) httpmiddleware.MiddlewareFunc {
	var config Config
	for _, opt := range opts {
		opt(&config)
	}

	return inertia.NewMiddleware(inertia.New(
		must.Must(vite.NewTemplate(
			RootTemplate,
			config.ViteConfig,
		)),
		&inertia.Config{
			RootViewID:  inertia.DefaultRootViewID,
			Concurrency: inertia.DefaultConcurrency,
			SSRClient:   config.SSRClient,
			JSONMarshalOptions: []json.Options{
				json.WithMarshalers(json.MarshalFunc(protojson.Marshal)),
			},
		},
	))
}

// Mount mounts a new inertia endpoint that uses protobuf messages for
// communication with client.
func Mount[M proto.Message](mux inertiaframe.Mux, endpoint Endpoint[M]) {
	inertiaframe.Mount(mux, endpoint, &inertiaframe.MountOpts[M]{
		Validator: inertiaframe.ValidatorFunc[M](func(data M) error {
			if err := protovalidate.Validate(data); err != nil {
				var verr *protovalidate.ValidationError
				if errors.As(err, &verr) {
					return convertValidationError(*verr)
				}

				return fmt.Errorf("failed request validation: %w", err)
			}

			return nil
		}),
		JSONUnmarshalOptions: []json.Options{json.WithUnmarshalers(
			json.UnmarshalFunc(protojson.Unmarshal),
		)},
	})
}

// convertValidationError converts a protobuf validation error to an inertia validation error.
func convertValidationError(verr protovalidate.ValidationError) inertia.ValidationErrors {
	var errs inertia.ValidationErrors
	for _, violation := range verr.Violations {
		errs = append(
			errs,
			inertia.NewValidationError(
				protovalidate.FieldPathString(violation.Proto.GetField()),
				violation.Proto.GetRuleId(),
			),
		)
	}

	return errs
}
