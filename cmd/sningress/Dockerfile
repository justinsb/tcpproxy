FROM golang:1.17 as builder

# Copy in the go src
WORKDIR /go/src/sningress

COPY go.mod go.mod
COPY go.sum go.sum
COPY cmd/sningress/go.mod cmd/sningress/go.mod 
COPY cmd/sningress/go.sum cmd/sningress/go.sum 
RUN go mod download
RUN cd ./cmd/sningress && CGO_ENABLED=0 go build k8s.io/apimachinery/pkg/apis/meta/v1 k8s.io/client-go/kubernetes k8s.io/client-go/tools/clientcmd k8s.io/klog/v2
COPY cmd/ cmd/
COPY pkg/ pkg/
COPY *.go ./

RUN cd ./cmd/sningress && CGO_ENABLED=0 go build -v -o /sningress-server .

FROM gcr.io/distroless/static-debian11:nonroot

COPY --from=builder /sningress-server /sningress-server
ENTRYPOINT ["/sningress-server"]