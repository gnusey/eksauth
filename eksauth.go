package eksauth

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

// Authenticate generates a REST configuration using STS and EKS which can used to create
// clients to interact with Kubernetes.
func Authenticate(ctx context.Context, eksc *eks.Client, stsc *sts.Client, cluster string) (*rest.Config, error) {
	info, err := eksc.DescribeCluster(ctx, &eks.DescribeClusterInput{
		Name: aws.String(cluster),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve cluster information: %w", err)
	}

	host, _, err := rest.DefaultServerURL(
		aws.ToString(info.Cluster.Endpoint), "", api.SchemeGroupVersion, true)
	if err != nil {
		return nil, fmt.Errorf("failed to generate cluster server url: %w", err)
	}

	cert, err := base64.StdEncoding.DecodeString(
		aws.ToString(info.Cluster.CertificateAuthority.Data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode cluster certificate authority data: %w", err)
	}

	preq, err := sts.NewPresignClient(stsc, func(o *sts.PresignOptions) {
		o.ClientOptions = []func(*sts.Options){
			func(o *sts.Options) {
				o.APIOptions = []func(*middleware.Stack) error{
					func(s *middleware.Stack) error {
						return s.Build.Add(middleware.BuildMiddlewareFunc("SetClusterId", func(ctx context.Context, in middleware.BuildInput, next middleware.BuildHandler) (middleware.BuildOutput, middleware.Metadata, error) {
							switch r := in.Request.(type) {
							case *smithyhttp.Request:
								r.Header.Add("x-k8s-aws-id", cluster)
								q := r.URL.Query()
								q.Set("X-Amz-Expires", strconv.FormatInt(int64(60/time.Second), 10))
								r.URL.RawQuery = q.Encode()
							default:
								return middleware.BuildOutput{},
									middleware.Metadata{},
									fmt.Errorf("unknown transport type %T", in.Request)
							}

							return next.HandleBuild(ctx, in)
						}), middleware.After)
					},
				}
			},
		}
	}).PresignGetCallerIdentity(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate sts request url: %w", err)
	}

	cfg, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(&clientcmd.ClientConfigLoadingRules{},
		&clientcmd.ConfigOverrides{
			ClusterInfo: api.Cluster{
				CertificateAuthorityData: cert,
				Server:                   host.String(),
			},
			AuthInfo: api.AuthInfo{
				Token: "k8s-aws-v1." + base64.RawURLEncoding.EncodeToString([]byte(preq.URL)),
			},
		}).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to create client config: %w", err)
	}

	return cfg, nil
}
