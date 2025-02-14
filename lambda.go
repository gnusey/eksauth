package eksauth

import (
	"context"
	"os"

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

// Environment variable keys.
const (
	EnvKeyCluster = "EKS_AUTH_CLUSTER"
	EnvKeyRegion  = "EKS_AUTH_REGION"
)

// LambdaHandlerOptions are used to change the behaviour of LambdaHandler.
type LambdaHandlerOptions struct {
	UseEnv          bool // Fill the option values using environment variables.
	Cluster, Region string
}

// LambdaHandlerWrap is a function to be executed when wrapped by a constructed Lambda
// handler, e.g:
//
//	lambda.Start(func(ctx context.Context, req request) (any, error) {
//		return eksauth.LambdaHandlerWrap(ctx, req, handler)
//	})
//
// This is useful when retaining the concrete type of `req` is required.
func LambdaHandlerWrap(ctx context.Context, req any, fn LambdaWrapFn, opts ...func(*LambdaHandlerOptions)) (any, error) {
	var opt LambdaHandlerOptions
	for _, n := range opts {
		n(&opt)
	}
	if opt.UseEnv {
		opt.Cluster = os.Getenv(EnvKeyCluster)
		opt.Region = os.Getenv(EnvKeyRegion)
	}

	cfg, err := config.LoadDefaultConfig(ctx,
		config.WithRegion(opt.Region))
	if err != nil {
		return nil, err
	}

	auth, err := Authenticate(ctx,
		eks.NewFromConfig(cfg), sts.NewFromConfig(cfg), opt.Cluster)
	if err != nil {
		return nil, err
	}

	return fn(ctx, req, cfg, auth)
}

// LambdaHandler returns a function that can be used as a Lambda handler. It authenticates
// with EKS using default IAM credentials and then executes the wrapped function. The input
// parameter of the Lambda will always be forced to `map[string]interface{}`.
func LambdaHandler(fn LambdaWrapFn, opts ...func(*LambdaHandlerOptions)) LambdaHandlerFn {
	return func(ctx context.Context, req any) (any, error) {
		return LambdaHandlerWrap(ctx, req, fn, opts...)
	}
}
