package main

import (
	"context"
	"math/big"
	"net"
	"regexp"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/fieldpath"
	"github.com/crossplane/crossplane-runtime/pkg/logging"
	"golang.org/x/exp/maps"
	"google.golang.org/protobuf/types/known/structpb"
	"k8s.io/apimachinery/pkg/util/intstr"

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

type LookupContext map[string]any

// RunFunction runs the Function.
func (f *Function) RunFunction(_ context.Context, req *fnv1beta1.RunFunctionRequest) (*fnv1beta1.RunFunctionResponse, error) {
	f.log.Info("Running function", "tag", req.GetMeta().GetTag())

	rsp := response.To(req, response.DefaultTTL)

	// Get inputs
	in := &v1beta1.Input{}
	if err := request.GetInput(req, in); err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot get Function input from %T", req))
		return rsp, nil
	}

	ctx := &LookupContext{
		"observed": req.Observed,
		"desired":  req.Desired,
		"context":  req.Context,
	}
	prefix, newbits, netnum, err := ResolveInputs(ctx, in)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot resolve inputs"))
		return rsp, nil
	}

	// Parse the base prefix
	_, cidr, err := net.ParseCIDR(prefix)
	if err != nil {
		response.Fatal(rsp, errors.Wrapf(err, "cannot parse subnet prefix"))
		return rsp, nil
	}

	// Validate the new subnet size
	ones, bits := cidr.Mask.Size()
	newPrefixLength := ones + newbits
	if newPrefixLength > bits {
		response.Fatal(rsp, errors.Errorf("not enough space to extend prefix %s by %d bits", prefix, newbits))
		return rsp, nil
	}

	if newbits < 64 && netnum > (1<<newbits)-1 {
		response.Fatal(rsp, errors.Errorf("netnum %d is greater than maximum allowed for %d newbits", netnum, newbits))
		return rsp, nil
	}

	// build the new subnet
	newPrefix := &big.Int{}
	newPrefix.SetBytes(cidr.IP)

	subnetNetnum := &big.Int{}
	subnetNetnum.SetInt64(int64(netnum))
	subnetNetnum.Lsh(subnetNetnum, uint(bits-newPrefixLength))
	newPrefix.Or(newPrefix, subnetNetnum)

	newPrefixBytes := make([]byte, bits/8)
	newPrefix.FillBytes(newPrefixBytes)

	subnet := &net.IPNet{
		IP:   net.IP(newPrefixBytes),
		Mask: net.CIDRMask(newPrefixLength, bits),
	}

	response.SetContextKey(rsp, in.Name, structpb.NewStringValue(subnet.String()))

	return rsp, nil
}

func ResolveInputs(ctx *LookupContext, in *v1beta1.Input) (prefix string, newbits int, netnum int, err error) {
	prefix, err = ctx.resolveString(in.Prefix)
	if err != nil {
		err = errors.Wrap(err, "cannot lookup subnet prefix")
		return
	}

	newbits = in.Newbits.IntValue()
	if newbits == 0 && in.Newbits.Type == intstr.String {
		newbits, err = ctx.resolveInt(in.Newbits.String())
		if err != nil {
			err = errors.Wrap(err, "cannot lookup subnet newbits")
			return
		}
	}

	netnum = in.Netnum.IntValue()
	if netnum == 0 && in.Netnum.Type == intstr.String {
		netnum, err = ctx.resolveInt(in.Netnum.String())
		if err != nil {
			err = errors.Wrap(err, "cannot lookup subnet netnum")
			return
		}
	}

	return
}

func (req *LookupContext) resolveString(input string) (output string, err error) {
	resolved, err := req.resolve(input)
	output = resolved.(string)
	return
}

func (req *LookupContext) resolveInt(input string) (output int, err error) {
	resolved, err := req.resolve(input)
	output = int(resolved.(int64))
	return
}

func (req *LookupContext) resolve(input string) (output any, err error) {
	r, _ := regexp.Compile(`^\$\{(.*)\}$`)
	if path := r.FindStringSubmatch(input); path != nil {
		err = req.lookup(path[1], &output)
		return
	}

	return input, nil
}

func (req LookupContext) lookup(path string, output any) error {
	segments, err := fieldpath.Parse(path)
	if err != nil {
		return errors.Wrapf(err, "failed to parse fieldpath %s", path)
	}

	lookup := segments[0].Field
	if _, ok := req[lookup]; !ok {
		return errors.Errorf("%s is not a supported lookup, expected one of %v", lookup, maps.Keys(req))
	}

	if lookup == "context" {
		paved := fieldpath.Pave(req[lookup].(*structpb.Struct).AsMap())
		return paved.GetValueInto(segments[1:].String(), output)
	}

	switch segments[1].Field {
	case "composite":
		paved := fieldpath.Pave(req[lookup].(*fnv1beta1.State).Composite.Resource.AsMap())
		return paved.GetValueInto(segments[2:].String(), output)
	case "resources":
		paved := fieldpath.Pave(req[lookup].(*fnv1beta1.State).Resources[segments[2].Field].Resource.AsMap())
		return paved.GetValueInto(segments[3:].String(), output)
	}

	return nil
}
