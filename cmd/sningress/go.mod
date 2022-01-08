module inet.af/tcpproxy/cmd/sningress

go 1.16

require (
	inet.af/tcpproxy v0.0.0
	k8s.io/apimachinery v0.23.1
	k8s.io/client-go v0.23.1
	k8s.io/klog/v2 v2.40.1
)

replace inet.af/tcpproxy => ../../
