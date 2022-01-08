FROM golang:1.17 as builder

# Copy in the go src
WORKDIR /go/src/sningress

COPY go.mod go.mod
COPY go.sum go.sum
COPY cmd/sningress/go.mod cmd/sningress/go.mod 
COPY cmd/sningress/go.sum cmd/sningress/go.sum 
RUN go mod download
RUN cd ./cmd/sningress && CGO_ENABLED=0 go build k8s.io/apimachinery/pkg/apis/meta/v1 k8s.io/client-go/kubernetes k8s.io/client-go/tools/clientcmd k8s.io/klog/v2 sigs.k8s.io/apiserver-network-proxy/konnectivity-client/pkg/client
COPY cmd/ cmd/
COPY pkg/ pkg/
COPY *.go ./

RUN cd ./cmd/sningress && CGO_ENABLED=0 go build -v -o /sningress-server .

#FROM gcr.io/distroless/static-debian11:nonroot
FROM gcr.io/distroless/static-debian11:latest

COPY --from=builder /sningress-server /sningress-server
ENTRYPOINT ["/sningress-server"]