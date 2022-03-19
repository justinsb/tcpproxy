FROM golang:1.18 as builder

# Copy in the go src
WORKDIR /go/src/sningress

COPY go.mod go.mod
COPY go.sum go.sum
COPY cmd/sningress/go.mod cmd/sningress/go.mod 
COPY cmd/sningress/go.sum cmd/sningress/go.sum 
RUN go mod download
RUN cd ./cmd/sningress && CGO_ENABLED=0 go build \
	k8s.io/apimachinery/pkg/types \
	k8s.io/client-go/rest \
	k8s.io/client-go/tools/clientcmd \
	k8s.io/klog/v2 \
    k8s.io/api/discovery/v1 \
	k8s.io/api/networking/v1 \
    k8s.io/apimachinery/pkg/apis/meta/v1 \
    google.golang.org/grpc
COPY cmd/ cmd/
COPY pkg/ pkg/
COPY *.go ./

RUN cd ./cmd/sningress && CGO_ENABLED=0 go build -v -o /sningress-server .

#FROM gcr.io/distroless/static-debian11:nonroot
FROM gcr.io/distroless/static-debian11:latest

COPY --from=builder /sningress-server /sningress-server
ENTRYPOINT ["/sningress-server"]