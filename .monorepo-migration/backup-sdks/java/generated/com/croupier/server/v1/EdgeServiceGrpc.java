package com.croupier.server.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * Server Edge Service - Internal interface for edge proxy job queries
 * </pre>
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class EdgeServiceGrpc {

  private EdgeServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "croupier.server.v1.EdgeService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<com.croupier.server.v1.GetJobResultRequest,
      com.croupier.server.v1.GetJobResultResponse> getGetJobResultMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetJobResult",
      requestType = com.croupier.server.v1.GetJobResultRequest.class,
      responseType = com.croupier.server.v1.GetJobResultResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.croupier.server.v1.GetJobResultRequest,
      com.croupier.server.v1.GetJobResultResponse> getGetJobResultMethod() {
    io.grpc.MethodDescriptor<com.croupier.server.v1.GetJobResultRequest, com.croupier.server.v1.GetJobResultResponse> getGetJobResultMethod;
    if ((getGetJobResultMethod = EdgeServiceGrpc.getGetJobResultMethod) == null) {
      synchronized (EdgeServiceGrpc.class) {
        if ((getGetJobResultMethod = EdgeServiceGrpc.getGetJobResultMethod) == null) {
          EdgeServiceGrpc.getGetJobResultMethod = getGetJobResultMethod =
              io.grpc.MethodDescriptor.<com.croupier.server.v1.GetJobResultRequest, com.croupier.server.v1.GetJobResultResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetJobResult"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.server.v1.GetJobResultRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.server.v1.GetJobResultResponse.getDefaultInstance()))
              .setSchemaDescriptor(new EdgeServiceMethodDescriptorSupplier("GetJobResult"))
              .build();
        }
      }
    }
    return getGetJobResultMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static EdgeServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<EdgeServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<EdgeServiceStub>() {
        @java.lang.Override
        public EdgeServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new EdgeServiceStub(channel, callOptions);
        }
      };
    return EdgeServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static EdgeServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<EdgeServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<EdgeServiceBlockingV2Stub>() {
        @java.lang.Override
        public EdgeServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new EdgeServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return EdgeServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static EdgeServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<EdgeServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<EdgeServiceBlockingStub>() {
        @java.lang.Override
        public EdgeServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new EdgeServiceBlockingStub(channel, callOptions);
        }
      };
    return EdgeServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static EdgeServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<EdgeServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<EdgeServiceFutureStub>() {
        @java.lang.Override
        public EdgeServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new EdgeServiceFutureStub(channel, callOptions);
        }
      };
    return EdgeServiceFutureStub.newStub(factory, channel);
  }

  /**
   * <pre>
   * Server Edge Service - Internal interface for edge proxy job queries
   * </pre>
   */
  public interface AsyncService {

    /**
     * <pre>
     * Get job result through edge proxy
     * </pre>
     */
    default void getJobResult(com.croupier.server.v1.GetJobResultRequest request,
        io.grpc.stub.StreamObserver<com.croupier.server.v1.GetJobResultResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetJobResultMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service EdgeService.
   * <pre>
   * Server Edge Service - Internal interface for edge proxy job queries
   * </pre>
   */
  public static abstract class EdgeServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return EdgeServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service EdgeService.
   * <pre>
   * Server Edge Service - Internal interface for edge proxy job queries
   * </pre>
   */
  public static final class EdgeServiceStub
      extends io.grpc.stub.AbstractAsyncStub<EdgeServiceStub> {
    private EdgeServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected EdgeServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new EdgeServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * Get job result through edge proxy
     * </pre>
     */
    public void getJobResult(com.croupier.server.v1.GetJobResultRequest request,
        io.grpc.stub.StreamObserver<com.croupier.server.v1.GetJobResultResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetJobResultMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service EdgeService.
   * <pre>
   * Server Edge Service - Internal interface for edge proxy job queries
   * </pre>
   */
  public static final class EdgeServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<EdgeServiceBlockingV2Stub> {
    private EdgeServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected EdgeServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new EdgeServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     * <pre>
     * Get job result through edge proxy
     * </pre>
     */
    public com.croupier.server.v1.GetJobResultResponse getJobResult(com.croupier.server.v1.GetJobResultRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetJobResultMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service EdgeService.
   * <pre>
   * Server Edge Service - Internal interface for edge proxy job queries
   * </pre>
   */
  public static final class EdgeServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<EdgeServiceBlockingStub> {
    private EdgeServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected EdgeServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new EdgeServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * Get job result through edge proxy
     * </pre>
     */
    public com.croupier.server.v1.GetJobResultResponse getJobResult(com.croupier.server.v1.GetJobResultRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetJobResultMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service EdgeService.
   * <pre>
   * Server Edge Service - Internal interface for edge proxy job queries
   * </pre>
   */
  public static final class EdgeServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<EdgeServiceFutureStub> {
    private EdgeServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected EdgeServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new EdgeServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * Get job result through edge proxy
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.croupier.server.v1.GetJobResultResponse> getJobResult(
        com.croupier.server.v1.GetJobResultRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetJobResultMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_GET_JOB_RESULT = 0;

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
        case METHODID_GET_JOB_RESULT:
          serviceImpl.getJobResult((com.croupier.server.v1.GetJobResultRequest) request,
              (io.grpc.stub.StreamObserver<com.croupier.server.v1.GetJobResultResponse>) responseObserver);
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
          getGetJobResultMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.croupier.server.v1.GetJobResultRequest,
              com.croupier.server.v1.GetJobResultResponse>(
                service, METHODID_GET_JOB_RESULT)))
        .build();
  }

  private static abstract class EdgeServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    EdgeServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return com.croupier.server.v1.EdgeProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("EdgeService");
    }
  }

  private static final class EdgeServiceFileDescriptorSupplier
      extends EdgeServiceBaseDescriptorSupplier {
    EdgeServiceFileDescriptorSupplier() {}
  }

  private static final class EdgeServiceMethodDescriptorSupplier
      extends EdgeServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    EdgeServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (EdgeServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new EdgeServiceFileDescriptorSupplier())
              .addMethod(getGetJobResultMethod())
              .build();
        }
      }
    }
    return result;
  }
}
