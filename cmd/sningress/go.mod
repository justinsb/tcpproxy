module inet.af/tcpproxy/cmd/sningress

go 1.16

require (
	google.golang.org/grpc v1.36.1
	inet.af/tcpproxy v0.0.0
	k8s.io/apimachinery v0.23.1
	k8s.io/client-go v0.23.1
	k8s.io/klog/v2 v2.40.1
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.0.27
)

replace inet.af/tcpproxy => ../../
