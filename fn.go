package main

import (
	"context"
	"math/big"
	"net"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"google.golang.org/protobuf/types/known/structpb"

	fnv1beta1 "github.com/crossplane/function-sdk-go/proto/v1beta1"
	"github.com/crossplane/function-sdk-go/request"
	"github.com/crossplane/function-sdk-go/response"

	"github.com/crossplane-contrib/function-cidrsubnet/input/v1beta1"
)

// Function returns whatever response you ask it to.
type Function struct {
	fnv1beta1.UnimplementedFunctionRunnerServiceServer

	log logging.Logger
}

// RunFunction runs the Function.
func (f *Function) RunFunction(_ context.Context, req *fnv1beta1.RunFunctionRequest) (*fnv1beta1.RunFunctionResponse, error) {
	f.log.Info("Running function", "tag", req.GetMeta().GetTag())

	rsp := response.To(req, response.DefaultTTL)

	in := &v1beta1.Input{}
	if err := request.GetInput(req, in); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get Function input from %T", req))
		return rsp, nil
	}

	_, cidr, err := net.ParseCIDR(in.Prefix)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot parse subnet prefix"))
		return rsp, nil
	}

	ones, bits := cidr.Mask.Size()
	newPrefixLength := ones + int(in.Newbits)
	if newPrefixLength > bits {
		response.Fatal(rsp, errors.Errorf("not enough space to extend prefix %s by %d bits", in.Prefix, in.Newbits))
		return rsp, nil
	}

	if in.Newbits < 64 && in.Netnum > (1<<in.Newbits)-1 {
		response.Fatal(rsp, errors.Errorf("netnum %d is greater than maximum allowed for %d newbits", in.Netnum, in.Newbits))
		return rsp, nil
	}

	newPrefix := &big.Int{}
	newPrefix.SetBytes(cidr.IP)

	netnum := &big.Int{}
	netnum.SetUint64(in.Netnum)
	netnum.Lsh(netnum, uint(bits-newPrefixLength))
	newPrefix.Or(newPrefix, netnum)

	newPrefixBytes := make([]byte, bits/8)
	newPrefix.FillBytes(newPrefixBytes)

	subnet := &net.IPNet{
		IP:   net.IP(newPrefixBytes),
		Mask: net.CIDRMask(newPrefixLength, bits),
	}

	response.SetContextKey(rsp, in.Name, structpb.NewStringValue(subnet.String()))

	return rsp, nil
}
