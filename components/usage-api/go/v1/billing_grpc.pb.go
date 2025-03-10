// Copyright (c) 2022 Gitpod GmbH. All rights reserved.
// Licensed under the GNU Affero General Public License (AGPL).
// See License-AGPL.txt in the project root for license information.

// Code generated by protoc-gen-go-grpc. DO NOT EDIT.
// versions:
// - protoc-gen-go-grpc v1.2.0
// - protoc             v3.20.1
// source: usage/v1/billing.proto

package v1

import (
	context "context"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
)

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
// Requires gRPC-Go v1.32.0 or later.
const _ = grpc.SupportPackageIsVersion7

// BillingServiceClient is the client API for BillingService service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://pkg.go.dev/google.golang.org/grpc/?tab=doc#ClientConn.NewStream.
type BillingServiceClient interface {
	// ReconcileInvoices retrieves current credit balance and reflects it in billing system.
	// Internal RPC, not intended for general consumption.
	ReconcileInvoices(ctx context.Context, in *ReconcileInvoicesRequest, opts ...grpc.CallOption) (*ReconcileInvoicesResponse, error)
	// GetUpcomingInvoice retrieves the latest invoice for a given query.
	GetUpcomingInvoice(ctx context.Context, in *GetUpcomingInvoiceRequest, opts ...grpc.CallOption) (*GetUpcomingInvoiceResponse, error)
	// FinalizeInvoice marks all sessions occurring in the given Stripe invoice as
	// having been invoiced.
	FinalizeInvoice(ctx context.Context, in *FinalizeInvoiceRequest, opts ...grpc.CallOption) (*FinalizeInvoiceResponse, error)
	// SetBilledSession marks an instance as billed with a billing system
	SetBilledSession(ctx context.Context, in *SetBilledSessionRequest, opts ...grpc.CallOption) (*SetBilledSessionResponse, error)
}

type billingServiceClient struct {
	cc grpc.ClientConnInterface
}

func NewBillingServiceClient(cc grpc.ClientConnInterface) BillingServiceClient {
	return &billingServiceClient{cc}
}

func (c *billingServiceClient) ReconcileInvoices(ctx context.Context, in *ReconcileInvoicesRequest, opts ...grpc.CallOption) (*ReconcileInvoicesResponse, error) {
	out := new(ReconcileInvoicesResponse)
	err := c.cc.Invoke(ctx, "/usage.v1.BillingService/ReconcileInvoices", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *billingServiceClient) GetUpcomingInvoice(ctx context.Context, in *GetUpcomingInvoiceRequest, opts ...grpc.CallOption) (*GetUpcomingInvoiceResponse, error) {
	out := new(GetUpcomingInvoiceResponse)
	err := c.cc.Invoke(ctx, "/usage.v1.BillingService/GetUpcomingInvoice", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *billingServiceClient) FinalizeInvoice(ctx context.Context, in *FinalizeInvoiceRequest, opts ...grpc.CallOption) (*FinalizeInvoiceResponse, error) {
	out := new(FinalizeInvoiceResponse)
	err := c.cc.Invoke(ctx, "/usage.v1.BillingService/FinalizeInvoice", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *billingServiceClient) SetBilledSession(ctx context.Context, in *SetBilledSessionRequest, opts ...grpc.CallOption) (*SetBilledSessionResponse, error) {
	out := new(SetBilledSessionResponse)
	err := c.cc.Invoke(ctx, "/usage.v1.BillingService/SetBilledSession", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// BillingServiceServer is the server API for BillingService service.
// All implementations must embed UnimplementedBillingServiceServer
// for forward compatibility
type BillingServiceServer interface {
	// ReconcileInvoices retrieves current credit balance and reflects it in billing system.
	// Internal RPC, not intended for general consumption.
	ReconcileInvoices(context.Context, *ReconcileInvoicesRequest) (*ReconcileInvoicesResponse, error)
	// GetUpcomingInvoice retrieves the latest invoice for a given query.
	GetUpcomingInvoice(context.Context, *GetUpcomingInvoiceRequest) (*GetUpcomingInvoiceResponse, error)
	// FinalizeInvoice marks all sessions occurring in the given Stripe invoice as
	// having been invoiced.
	FinalizeInvoice(context.Context, *FinalizeInvoiceRequest) (*FinalizeInvoiceResponse, error)
	// SetBilledSession marks an instance as billed with a billing system
	SetBilledSession(context.Context, *SetBilledSessionRequest) (*SetBilledSessionResponse, error)
	mustEmbedUnimplementedBillingServiceServer()
}

// UnimplementedBillingServiceServer must be embedded to have forward compatible implementations.
type UnimplementedBillingServiceServer struct {
}

func (UnimplementedBillingServiceServer) ReconcileInvoices(context.Context, *ReconcileInvoicesRequest) (*ReconcileInvoicesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method ReconcileInvoices not implemented")
}
func (UnimplementedBillingServiceServer) GetUpcomingInvoice(context.Context, *GetUpcomingInvoiceRequest) (*GetUpcomingInvoiceResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetUpcomingInvoice not implemented")
}
func (UnimplementedBillingServiceServer) FinalizeInvoice(context.Context, *FinalizeInvoiceRequest) (*FinalizeInvoiceResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method FinalizeInvoice not implemented")
}
func (UnimplementedBillingServiceServer) SetBilledSession(context.Context, *SetBilledSessionRequest) (*SetBilledSessionResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetBilledSession not implemented")
}
func (UnimplementedBillingServiceServer) mustEmbedUnimplementedBillingServiceServer() {}

// UnsafeBillingServiceServer may be embedded to opt out of forward compatibility for this service.
// Use of this interface is not recommended, as added methods to BillingServiceServer will
// result in compilation errors.
type UnsafeBillingServiceServer interface {
	mustEmbedUnimplementedBillingServiceServer()
}

func RegisterBillingServiceServer(s grpc.ServiceRegistrar, srv BillingServiceServer) {
	s.RegisterService(&BillingService_ServiceDesc, srv)
}

func _BillingService_ReconcileInvoices_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ReconcileInvoicesRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BillingServiceServer).ReconcileInvoices(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/usage.v1.BillingService/ReconcileInvoices",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BillingServiceServer).ReconcileInvoices(ctx, req.(*ReconcileInvoicesRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _BillingService_GetUpcomingInvoice_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(GetUpcomingInvoiceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BillingServiceServer).GetUpcomingInvoice(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/usage.v1.BillingService/GetUpcomingInvoice",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BillingServiceServer).GetUpcomingInvoice(ctx, req.(*GetUpcomingInvoiceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _BillingService_FinalizeInvoice_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(FinalizeInvoiceRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BillingServiceServer).FinalizeInvoice(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/usage.v1.BillingService/FinalizeInvoice",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BillingServiceServer).FinalizeInvoice(ctx, req.(*FinalizeInvoiceRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _BillingService_SetBilledSession_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(SetBilledSessionRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(BillingServiceServer).SetBilledSession(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/usage.v1.BillingService/SetBilledSession",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(BillingServiceServer).SetBilledSession(ctx, req.(*SetBilledSessionRequest))
	}
	return interceptor(ctx, in, info, handler)
}

// BillingService_ServiceDesc is the grpc.ServiceDesc for BillingService service.
// It's only intended for direct use with grpc.RegisterService,
// and not to be introspected or modified (even as a copy)
var BillingService_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "usage.v1.BillingService",
	HandlerType: (*BillingServiceServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "ReconcileInvoices",
			Handler:    _BillingService_ReconcileInvoices_Handler,
		},
		{
			MethodName: "GetUpcomingInvoice",
			Handler:    _BillingService_GetUpcomingInvoice_Handler,
		},
		{
			MethodName: "FinalizeInvoice",
			Handler:    _BillingService_FinalizeInvoice_Handler,
		},
		{
			MethodName: "SetBilledSession",
			Handler:    _BillingService_SetBilledSession_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "usage/v1/billing.proto",
}
