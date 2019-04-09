package objectstatus

import (
	"context"
	"testing"

	"github.com/golang/mock/gomock"
	storefake "github.com/heptio/developer-dash/internal/objectstore/fake"
	"github.com/heptio/developer-dash/internal/testutil"
	"github.com/heptio/developer-dash/pkg/cacheutil"
	"github.com/heptio/developer-dash/pkg/view/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func Test_runIngressStatus(t *testing.T) {
	cases := []struct {
		name     string
		init     func(*testing.T, *storefake.MockObjectStore) runtime.Object
		expected ObjectStatus
		isErr    bool
	}{
		{
			name: "in general",
			init: func(t *testing.T, o *storefake.MockObjectStore) runtime.Object {
				mockServiceInCache(t, o, "default", "single-service", "service_single_service.yaml")
				objectFile := "ingress_single_service.yaml"
				return testutil.LoadObjectFromFile(t, objectFile)

			},
			expected: ObjectStatus{
				Details: component.TitleFromString("Ingress is OK"),
			},
		},
		{
			name: "no matching backends",
			init: func(t *testing.T, o *storefake.MockObjectStore) runtime.Object {
				key := cacheutil.Key{Namespace: "default", APIVersion: "v1", Kind: "Service", Name: "no-such-service"}
				o.EXPECT().Get(gomock.Any(), gomock.Eq(key)).Return(nil, nil)

				objectFile := "ingress_no_matching_backend.yaml"
				return testutil.LoadObjectFromFile(t, objectFile)

			},
			expected: ObjectStatus{
				nodeStatus: component.NodeStatusError,
				Details:    component.TitleFromString("Backend refers to service \"no-such-service\" which doesn't exist"),
			},
		},
		{
			name: "no matching port",
			init: func(t *testing.T, o *storefake.MockObjectStore) runtime.Object {
				mockServiceInCache(t, o, "default", "service-wrong-port", "service_wrong_port.yaml")
				objectFile := "ingress_no_matching_port.yaml"
				return testutil.LoadObjectFromFile(t, objectFile)
			},
			expected: ObjectStatus{
				nodeStatus: component.NodeStatusError,
				Details:    component.TitleFromString("Backend for service \"service-wrong-port\" specifies an invalid port"),
			},
		},
		{
			name: "mismatched TLS host",
			init: func(t *testing.T, o *storefake.MockObjectStore) runtime.Object {
				mockServiceInCache(t, o, "default", "my-service", "service_my-service.yaml")
				mockSecretInCache(t, o, "default", "testsecret-tls", "secret_testsecret-tls.yaml")

				objectFile := "ingress_mismatched_tls_host.yaml"
				return testutil.LoadObjectFromFile(t, objectFile)

			},
			expected: ObjectStatus{
				nodeStatus: component.NodeStatusError,
				Details:    component.TitleFromString("No matching TLS host for rule \"not-the-tls-host.com\""),
			},
		},
		{
			name: "missing TLS secret",
			init: func(t *testing.T, o *storefake.MockObjectStore) runtime.Object {
				mockServiceInCache(t, o, "default", "my-service", "service_my-service.yaml")

				key := cacheutil.Key{Namespace: "default", APIVersion: "v1", Kind: "Secret", Name: "no-such-secret"}
				o.EXPECT().Get(gomock.Any(), gomock.Eq(key)).Return(nil, nil)

				objectFile := "ingress_ingress-bad-tls-host.yaml"
				return testutil.LoadObjectFromFile(t, objectFile)

			},
			expected: ObjectStatus{
				nodeStatus: component.NodeStatusError,
				Details:    component.TitleFromString("Secret \"no-such-secret\" does not exist"),
			},
		},
		{
			name: "object is nil",
			init: func(t *testing.T, o *storefake.MockObjectStore) runtime.Object {
				return nil
			},
			isErr: true,
		},
		{
			name: "object is not an ingress",
			init: func(t *testing.T, o *storefake.MockObjectStore) runtime.Object {
				return &unstructured.Unstructured{}
			},
			isErr: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			controller := gomock.NewController(t)
			defer controller.Finish()

			o := storefake.NewMockObjectStore(controller)

			object := tc.init(t, o)

			ctx := context.Background()
			status, err := runIngressStatus(ctx, object, o)
			if tc.isErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)

			assert.Equal(t, tc.expected, status)
		})
	}
}

func mockSecretInCache(t *testing.T, o *storefake.MockObjectStore, namespace, name, file string) runtime.Object {
	secret := testutil.LoadObjectFromFile(t, file)
	key := cacheutil.Key{
		Namespace:  namespace,
		APIVersion: "v1",
		Kind:       "Secret",
		Name:       name,
	}

	o.EXPECT().Get(gomock.Any(), gomock.Eq(key)).Return(testutil.ToUnstructured(t, secret), nil)

	return secret
}

func mockServiceInCache(t *testing.T, o *storefake.MockObjectStore, namespace, name, file string) runtime.Object {
	secret := testutil.LoadObjectFromFile(t, file)
	key := cacheutil.Key{
		Namespace:  namespace,
		APIVersion: "v1",
		Kind:       "Service",
		Name:       name,
	}

	o.EXPECT().Get(gomock.Any(), gomock.Eq(key)).Return(testutil.ToUnstructured(t, secret), nil)

	return secret
}
