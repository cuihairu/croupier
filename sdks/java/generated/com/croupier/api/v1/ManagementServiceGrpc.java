package com.croupier.api.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * Public Management Service (placeholder)
 * </pre>
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class ManagementServiceGrpc {

  private ManagementServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "croupier.api.v1.ManagementService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<com.croupier.api.v1.ManagementRequest,
      com.croupier.api.v1.ManagementResponse> getManageMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "Manage",
      requestType = com.croupier.api.v1.ManagementRequest.class,
      responseType = com.croupier.api.v1.ManagementResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.croupier.api.v1.ManagementRequest,
      com.croupier.api.v1.ManagementResponse> getManageMethod() {
    io.grpc.MethodDescriptor<com.croupier.api.v1.ManagementRequest, com.croupier.api.v1.ManagementResponse> getManageMethod;
    if ((getManageMethod = ManagementServiceGrpc.getManageMethod) == null) {
      synchronized (ManagementServiceGrpc.class) {
        if ((getManageMethod = ManagementServiceGrpc.getManageMethod) == null) {
          ManagementServiceGrpc.getManageMethod = getManageMethod =
              io.grpc.MethodDescriptor.<com.croupier.api.v1.ManagementRequest, com.croupier.api.v1.ManagementResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "Manage"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.api.v1.ManagementRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.api.v1.ManagementResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ManagementServiceMethodDescriptorSupplier("Manage"))
              .build();
        }
      }
    }
    return getManageMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static ManagementServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ManagementServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ManagementServiceStub>() {
        @java.lang.Override
        public ManagementServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ManagementServiceStub(channel, callOptions);
        }
      };
    return ManagementServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static ManagementServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ManagementServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ManagementServiceBlockingV2Stub>() {
        @java.lang.Override
        public ManagementServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ManagementServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return ManagementServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static ManagementServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ManagementServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ManagementServiceBlockingStub>() {
        @java.lang.Override
        public ManagementServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ManagementServiceBlockingStub(channel, callOptions);
        }
      };
    return ManagementServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static ManagementServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ManagementServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ManagementServiceFutureStub>() {
        @java.lang.Override
        public ManagementServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ManagementServiceFutureStub(channel, callOptions);
        }
      };
    return ManagementServiceFutureStub.newStub(factory, channel);
  }

  /**
   * <pre>
   * Public Management Service (placeholder)
   * </pre>
   */
  public interface AsyncService {

    /**
     * <pre>
     * Future: HTTP REST API management endpoint
     * </pre>
     */
    default void manage(com.croupier.api.v1.ManagementRequest request,
        io.grpc.stub.StreamObserver<com.croupier.api.v1.ManagementResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getManageMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service ManagementService.
   * <pre>
   * Public Management Service (placeholder)
   * </pre>
   */
  public static abstract class ManagementServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return ManagementServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service ManagementService.
   * <pre>
   * Public Management Service (placeholder)
   * </pre>
   */
  public static final class ManagementServiceStub
      extends io.grpc.stub.AbstractAsyncStub<ManagementServiceStub> {
    private ManagementServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ManagementServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ManagementServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * Future: HTTP REST API management endpoint
     * </pre>
     */
    public void manage(com.croupier.api.v1.ManagementRequest request,
        io.grpc.stub.StreamObserver<com.croupier.api.v1.ManagementResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getManageMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service ManagementService.
   * <pre>
   * Public Management Service (placeholder)
   * </pre>
   */
  public static final class ManagementServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<ManagementServiceBlockingV2Stub> {
    private ManagementServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ManagementServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ManagementServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     * <pre>
     * Future: HTTP REST API management endpoint
     * </pre>
     */
    public com.croupier.api.v1.ManagementResponse manage(com.croupier.api.v1.ManagementRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getManageMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service ManagementService.
   * <pre>
   * Public Management Service (placeholder)
   * </pre>
   */
  public static final class ManagementServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<ManagementServiceBlockingStub> {
    private ManagementServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ManagementServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ManagementServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * Future: HTTP REST API management endpoint
     * </pre>
     */
    public com.croupier.api.v1.ManagementResponse manage(com.croupier.api.v1.ManagementRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getManageMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service ManagementService.
   * <pre>
   * Public Management Service (placeholder)
   * </pre>
   */
  public static final class ManagementServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<ManagementServiceFutureStub> {
    private ManagementServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ManagementServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ManagementServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * Future: HTTP REST API management endpoint
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.croupier.api.v1.ManagementResponse> manage(
        com.croupier.api.v1.ManagementRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getManageMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_MANAGE = 0;

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
        case METHODID_MANAGE:
          serviceImpl.manage((com.croupier.api.v1.ManagementRequest) request,
              (io.grpc.stub.StreamObserver<com.croupier.api.v1.ManagementResponse>) responseObserver);
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
          getManageMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.croupier.api.v1.ManagementRequest,
              com.croupier.api.v1.ManagementResponse>(
                service, METHODID_MANAGE)))
        .build();
  }

  private static abstract class ManagementServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    ManagementServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return com.croupier.api.v1.ManagementProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("ManagementService");
    }
  }

  private static final class ManagementServiceFileDescriptorSupplier
      extends ManagementServiceBaseDescriptorSupplier {
    ManagementServiceFileDescriptorSupplier() {}
  }

  private static final class ManagementServiceMethodDescriptorSupplier
      extends ManagementServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    ManagementServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (ManagementServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new ManagementServiceFileDescriptorSupplier())
              .addMethod(getManageMethod())
              .build();
        }
      }
    }
    return result;
  }
}
