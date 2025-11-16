package com.examples.integrations.prom.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class PromGmServiceGrpc {

  private PromGmServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "examples.integrations.prom.v1.PromGmService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<com.examples.integrations.prom.v1.QueryRangeRequest,
      com.examples.integrations.prom.v1.QueryRangeResponse> getQueryRangeMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "QueryRange",
      requestType = com.examples.integrations.prom.v1.QueryRangeRequest.class,
      responseType = com.examples.integrations.prom.v1.QueryRangeResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.examples.integrations.prom.v1.QueryRangeRequest,
      com.examples.integrations.prom.v1.QueryRangeResponse> getQueryRangeMethod() {
    io.grpc.MethodDescriptor<com.examples.integrations.prom.v1.QueryRangeRequest, com.examples.integrations.prom.v1.QueryRangeResponse> getQueryRangeMethod;
    if ((getQueryRangeMethod = PromGmServiceGrpc.getQueryRangeMethod) == null) {
      synchronized (PromGmServiceGrpc.class) {
        if ((getQueryRangeMethod = PromGmServiceGrpc.getQueryRangeMethod) == null) {
          PromGmServiceGrpc.getQueryRangeMethod = getQueryRangeMethod =
              io.grpc.MethodDescriptor.<com.examples.integrations.prom.v1.QueryRangeRequest, com.examples.integrations.prom.v1.QueryRangeResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "QueryRange"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.examples.integrations.prom.v1.QueryRangeRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.examples.integrations.prom.v1.QueryRangeResponse.getDefaultInstance()))
              .setSchemaDescriptor(new PromGmServiceMethodDescriptorSupplier("QueryRange"))
              .build();
        }
      }
    }
    return getQueryRangeMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static PromGmServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PromGmServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PromGmServiceStub>() {
        @java.lang.Override
        public PromGmServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PromGmServiceStub(channel, callOptions);
        }
      };
    return PromGmServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static PromGmServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PromGmServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PromGmServiceBlockingV2Stub>() {
        @java.lang.Override
        public PromGmServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PromGmServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return PromGmServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static PromGmServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PromGmServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PromGmServiceBlockingStub>() {
        @java.lang.Override
        public PromGmServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PromGmServiceBlockingStub(channel, callOptions);
        }
      };
    return PromGmServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static PromGmServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PromGmServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PromGmServiceFutureStub>() {
        @java.lang.Override
        public PromGmServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PromGmServiceFutureStub(channel, callOptions);
        }
      };
    return PromGmServiceFutureStub.newStub(factory, channel);
  }

  /**
   */
  public interface AsyncService {

    /**
     */
    default void queryRange(com.examples.integrations.prom.v1.QueryRangeRequest request,
        io.grpc.stub.StreamObserver<com.examples.integrations.prom.v1.QueryRangeResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getQueryRangeMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service PromGmService.
   */
  public static abstract class PromGmServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return PromGmServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service PromGmService.
   */
  public static final class PromGmServiceStub
      extends io.grpc.stub.AbstractAsyncStub<PromGmServiceStub> {
    private PromGmServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PromGmServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PromGmServiceStub(channel, callOptions);
    }

    /**
     */
    public void queryRange(com.examples.integrations.prom.v1.QueryRangeRequest request,
        io.grpc.stub.StreamObserver<com.examples.integrations.prom.v1.QueryRangeResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getQueryRangeMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service PromGmService.
   */
  public static final class PromGmServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<PromGmServiceBlockingV2Stub> {
    private PromGmServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PromGmServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PromGmServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     */
    public com.examples.integrations.prom.v1.QueryRangeResponse queryRange(com.examples.integrations.prom.v1.QueryRangeRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getQueryRangeMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service PromGmService.
   */
  public static final class PromGmServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<PromGmServiceBlockingStub> {
    private PromGmServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PromGmServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PromGmServiceBlockingStub(channel, callOptions);
    }

    /**
     */
    public com.examples.integrations.prom.v1.QueryRangeResponse queryRange(com.examples.integrations.prom.v1.QueryRangeRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getQueryRangeMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service PromGmService.
   */
  public static final class PromGmServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<PromGmServiceFutureStub> {
    private PromGmServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PromGmServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PromGmServiceFutureStub(channel, callOptions);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.examples.integrations.prom.v1.QueryRangeResponse> queryRange(
        com.examples.integrations.prom.v1.QueryRangeRequest request) {
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
          serviceImpl.queryRange((com.examples.integrations.prom.v1.QueryRangeRequest) request,
              (io.grpc.stub.StreamObserver<com.examples.integrations.prom.v1.QueryRangeResponse>) responseObserver);
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
              com.examples.integrations.prom.v1.QueryRangeRequest,
              com.examples.integrations.prom.v1.QueryRangeResponse>(
                service, METHODID_QUERY_RANGE)))
        .build();
  }

  private static abstract class PromGmServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    PromGmServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return com.examples.integrations.prom.v1.PromProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("PromGmService");
    }
  }

  private static final class PromGmServiceFileDescriptorSupplier
      extends PromGmServiceBaseDescriptorSupplier {
    PromGmServiceFileDescriptorSupplier() {}
  }

  private static final class PromGmServiceMethodDescriptorSupplier
      extends PromGmServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    PromGmServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (PromGmServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new PromGmServiceFileDescriptorSupplier())
              .addMethod(getQueryRangeMethod())
              .build();
        }
      }
    }
    return result;
  }
}
