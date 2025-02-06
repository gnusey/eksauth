package eksauth

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"k8s.io/client-go/rest"
)

// LambdaHandlerFn is the signature for a Lambda handler.
type LambdaHandlerFn func(context.Context, any) (any, error)

// LambdaWrapFn is the signature for functions executed by LambdaHandler.
type LambdaWrapFn func(context.Context, any, aws.Config, *rest.Config) (any, error)

// LambdaHandler returns a function that can be used as a Lambda handler. It authenticates
// with EKS using default IAM credentials and then executes the wrapped function.
func LambdaHandler(cluster, region string, fn LambdaWrapFn) LambdaHandlerFn {
	return func(ctx context.Context, req any) (any, error) {
		cfg, err := config.LoadDefaultConfig(ctx,
			config.WithRegion(region))
		if err != nil {
			return nil, err
		}

		auth, err := Authenticate(ctx,
			eks.NewFromConfig(cfg), sts.NewFromConfig(cfg), cluster)
		if err != nil {
			return nil, err
		}

		return fn(ctx, req, cfg, auth)
	}
}
