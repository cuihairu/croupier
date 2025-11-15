package com.croupier.integrations.prom.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class PromGmGrpc {

  private PromGmGrpc() {}

  public static final java.lang.String SERVICE_NAME = "croupier.integrations.prom.v1.PromGm";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<com.croupier.integrations.prom.v1.QueryRangeRequest,
      com.croupier.integrations.prom.v1.QueryRangeResponse> getQueryRangeMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "QueryRange",
      requestType = com.croupier.integrations.prom.v1.QueryRangeRequest.class,
      responseType = com.croupier.integrations.prom.v1.QueryRangeResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.croupier.integrations.prom.v1.QueryRangeRequest,
      com.croupier.integrations.prom.v1.QueryRangeResponse> getQueryRangeMethod() {
    io.grpc.MethodDescriptor<com.croupier.integrations.prom.v1.QueryRangeRequest, com.croupier.integrations.prom.v1.QueryRangeResponse> getQueryRangeMethod;
    if ((getQueryRangeMethod = PromGmGrpc.getQueryRangeMethod) == null) {
      synchronized (PromGmGrpc.class) {
        if ((getQueryRangeMethod = PromGmGrpc.getQueryRangeMethod) == null) {
          PromGmGrpc.getQueryRangeMethod = getQueryRangeMethod =
              io.grpc.MethodDescriptor.<com.croupier.integrations.prom.v1.QueryRangeRequest, com.croupier.integrations.prom.v1.QueryRangeResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "QueryRange"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.integrations.prom.v1.QueryRangeRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.integrations.prom.v1.QueryRangeResponse.getDefaultInstance()))
              .setSchemaDescriptor(new PromGmMethodDescriptorSupplier("QueryRange"))
              .build();
        }
      }
    }
    return getQueryRangeMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static PromGmStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PromGmStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PromGmStub>() {
        @java.lang.Override
        public PromGmStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PromGmStub(channel, callOptions);
        }
      };
    return PromGmStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static PromGmBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PromGmBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PromGmBlockingV2Stub>() {
        @java.lang.Override
        public PromGmBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PromGmBlockingV2Stub(channel, callOptions);
        }
      };
    return PromGmBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static PromGmBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PromGmBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PromGmBlockingStub>() {
        @java.lang.Override
        public PromGmBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PromGmBlockingStub(channel, callOptions);
        }
      };
    return PromGmBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static PromGmFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PromGmFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PromGmFutureStub>() {
        @java.lang.Override
        public PromGmFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PromGmFutureStub(channel, callOptions);
        }
      };
    return PromGmFutureStub.newStub(factory, channel);
  }

  /**
   */
  public interface AsyncService {

    /**
     */
    default void queryRange(com.croupier.integrations.prom.v1.QueryRangeRequest request,
        io.grpc.stub.StreamObserver<com.croupier.integrations.prom.v1.QueryRangeResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getQueryRangeMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service PromGm.
   */
  public static abstract class PromGmImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return PromGmGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service PromGm.
   */
  public static final class PromGmStub
      extends io.grpc.stub.AbstractAsyncStub<PromGmStub> {
    private PromGmStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PromGmStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PromGmStub(channel, callOptions);
    }

    /**
     */
    public void queryRange(com.croupier.integrations.prom.v1.QueryRangeRequest request,
        io.grpc.stub.StreamObserver<com.croupier.integrations.prom.v1.QueryRangeResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getQueryRangeMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service PromGm.
   */
  public static final class PromGmBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<PromGmBlockingV2Stub> {
    private PromGmBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PromGmBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PromGmBlockingV2Stub(channel, callOptions);
    }

    /**
     */
    public com.croupier.integrations.prom.v1.QueryRangeResponse queryRange(com.croupier.integrations.prom.v1.QueryRangeRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getQueryRangeMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service PromGm.
   */
  public static final class PromGmBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<PromGmBlockingStub> {
    private PromGmBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PromGmBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PromGmBlockingStub(channel, callOptions);
    }

    /**
     */
    public com.croupier.integrations.prom.v1.QueryRangeResponse queryRange(com.croupier.integrations.prom.v1.QueryRangeRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getQueryRangeMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service PromGm.
   */
  public static final class PromGmFutureStub
      extends io.grpc.stub.AbstractFutureStub<PromGmFutureStub> {
    private PromGmFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PromGmFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PromGmFutureStub(channel, callOptions);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.croupier.integrations.prom.v1.QueryRangeResponse> queryRange(
        com.croupier.integrations.prom.v1.QueryRangeRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getQueryRangeMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_QUERY_RANGE = 0;

  private static final class MethodHandlers<Req, Resp> implements
      io.grpc.stub.ServerCalls.UnaryMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.ServerStreamingMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.ClientStreamingMethod<Req, Resp>,
      io.grpc.stub.ServerCalls.BidiStreamingMethod<Req, Resp> {
    private final AsyncService serviceImpl;
    private final int methodId;

    MethodHandlers(AsyncService serviceImpl, int methodId) {
      this.serviceImpl = serviceImpl;
      this.methodId = methodId;
    }

    @java.lang.Override
    @java.lang.SuppressWarnings("unchecked")
    public void invoke(Req request, io.grpc.stub.StreamObserver<Resp> responseObserver) {
      switch (methodId) {
        case METHODID_QUERY_RANGE:
          serviceImpl.queryRange((com.croupier.integrations.prom.v1.QueryRangeRequest) request,
              (io.grpc.stub.StreamObserver<com.croupier.integrations.prom.v1.QueryRangeResponse>) responseObserver);
          break;
        default:
          throw new AssertionError();
      }
    }

    @java.lang.Override
    @java.lang.SuppressWarnings("unchecked")
    public io.grpc.stub.StreamObserver<Req> invoke(
        io.grpc.stub.StreamObserver<Resp> responseObserver) {
      switch (methodId) {
        default:
          throw new AssertionError();
      }
    }
  }

  public static final io.grpc.ServerServiceDefinition bindService(AsyncService service) {
    return io.grpc.ServerServiceDefinition.builder(getServiceDescriptor())
        .addMethod(
          getQueryRangeMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.croupier.integrations.prom.v1.QueryRangeRequest,
              com.croupier.integrations.prom.v1.QueryRangeResponse>(
                service, METHODID_QUERY_RANGE)))
        .build();
  }

  private static abstract class PromGmBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    PromGmBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return com.croupier.integrations.prom.v1.PromProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("PromGm");
    }
  }

  private static final class PromGmFileDescriptorSupplier
      extends PromGmBaseDescriptorSupplier {
    PromGmFileDescriptorSupplier() {}
  }

  private static final class PromGmMethodDescriptorSupplier
      extends PromGmBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    PromGmMethodDescriptorSupplier(java.lang.String methodName) {
      this.methodName = methodName;
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.MethodDescriptor getMethodDescriptor() {
      return getServiceDescriptor().findMethodByName(methodName);
    }
  }

  private static volatile io.grpc.ServiceDescriptor serviceDescriptor;

  public static io.grpc.ServiceDescriptor getServiceDescriptor() {
    io.grpc.ServiceDescriptor result = serviceDescriptor;
    if (result == null) {
      synchronized (PromGmGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new PromGmFileDescriptorSupplier())
              .addMethod(getQueryRangeMethod())
              .build();
        }
      }
    }
    return result;
  }
}
