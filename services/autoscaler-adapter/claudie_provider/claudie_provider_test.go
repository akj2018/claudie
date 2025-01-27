package claudie_provider

import (
	"context"
	"testing"

	"github.com/rs/zerolog/log"
	"google.golang.org/grpc"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider/externalgrpc/protos"

	"github.com/berops/claudie/internal/utils"
)

// TestRefresh tests autoscaler adapter running in local environment.
func TestRefresh(t *testing.T) {
	var cc *grpc.ClientConn
	var err error
	URL := "localhost:50000"

	if cc, err = utils.GrpcDialWithInsecure("adapter", URL); err != nil {
		t.Error(err)
	}

	defer func() {
		if err := cc.Close(); err != nil {
			log.Err(err).Msgf("Failed to close adapter connection")
		}
	}()

	c := protos.NewCloudProviderClient(cc)
	if _, err = c.Refresh(context.Background(), &protos.RefreshRequest{}); err != nil {
		t.Error(err)
	}
}
