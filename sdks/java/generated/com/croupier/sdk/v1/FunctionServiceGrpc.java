package com.croupier.sdk.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * SDK Function Service - Main interface for invoking functions
 * This is what SDK developers will use to call game functions
 * </pre>
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class FunctionServiceGrpc {

  private FunctionServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "croupier.sdk.v1.FunctionService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<com.croupier.sdk.v1.InvokeRequest,
      com.croupier.sdk.v1.InvokeResponse> getInvokeMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "Invoke",
      requestType = com.croupier.sdk.v1.InvokeRequest.class,
      responseType = com.croupier.sdk.v1.InvokeResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.croupier.sdk.v1.InvokeRequest,
      com.croupier.sdk.v1.InvokeResponse> getInvokeMethod() {
    io.grpc.MethodDescriptor<com.croupier.sdk.v1.InvokeRequest, com.croupier.sdk.v1.InvokeResponse> getInvokeMethod;
    if ((getInvokeMethod = FunctionServiceGrpc.getInvokeMethod) == null) {
      synchronized (FunctionServiceGrpc.class) {
        if ((getInvokeMethod = FunctionServiceGrpc.getInvokeMethod) == null) {
          FunctionServiceGrpc.getInvokeMethod = getInvokeMethod =
              io.grpc.MethodDescriptor.<com.croupier.sdk.v1.InvokeRequest, com.croupier.sdk.v1.InvokeResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "Invoke"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.sdk.v1.InvokeRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.sdk.v1.InvokeResponse.getDefaultInstance()))
              .setSchemaDescriptor(new FunctionServiceMethodDescriptorSupplier("Invoke"))
              .build();
        }
      }
    }
    return getInvokeMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.croupier.sdk.v1.InvokeRequest,
      com.croupier.sdk.v1.StartJobResponse> getStartJobMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "StartJob",
      requestType = com.croupier.sdk.v1.InvokeRequest.class,
      responseType = com.croupier.sdk.v1.StartJobResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.croupier.sdk.v1.InvokeRequest,
      com.croupier.sdk.v1.StartJobResponse> getStartJobMethod() {
    io.grpc.MethodDescriptor<com.croupier.sdk.v1.InvokeRequest, com.croupier.sdk.v1.StartJobResponse> getStartJobMethod;
    if ((getStartJobMethod = FunctionServiceGrpc.getStartJobMethod) == null) {
      synchronized (FunctionServiceGrpc.class) {
        if ((getStartJobMethod = FunctionServiceGrpc.getStartJobMethod) == null) {
          FunctionServiceGrpc.getStartJobMethod = getStartJobMethod =
              io.grpc.MethodDescriptor.<com.croupier.sdk.v1.InvokeRequest, com.croupier.sdk.v1.StartJobResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "StartJob"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.sdk.v1.InvokeRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.sdk.v1.StartJobResponse.getDefaultInstance()))
              .setSchemaDescriptor(new FunctionServiceMethodDescriptorSupplier("StartJob"))
              .build();
        }
      }
    }
    return getStartJobMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.croupier.sdk.v1.JobStreamRequest,
      com.croupier.sdk.v1.JobEvent> getStreamJobMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "StreamJob",
      requestType = com.croupier.sdk.v1.JobStreamRequest.class,
      responseType = com.croupier.sdk.v1.JobEvent.class,
      methodType = io.grpc.MethodDescriptor.MethodType.SERVER_STREAMING)
  public static io.grpc.MethodDescriptor<com.croupier.sdk.v1.JobStreamRequest,
      com.croupier.sdk.v1.JobEvent> getStreamJobMethod() {
    io.grpc.MethodDescriptor<com.croupier.sdk.v1.JobStreamRequest, com.croupier.sdk.v1.JobEvent> getStreamJobMethod;
    if ((getStreamJobMethod = FunctionServiceGrpc.getStreamJobMethod) == null) {
      synchronized (FunctionServiceGrpc.class) {
        if ((getStreamJobMethod = FunctionServiceGrpc.getStreamJobMethod) == null) {
          FunctionServiceGrpc.getStreamJobMethod = getStreamJobMethod =
              io.grpc.MethodDescriptor.<com.croupier.sdk.v1.JobStreamRequest, com.croupier.sdk.v1.JobEvent>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.SERVER_STREAMING)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "StreamJob"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.sdk.v1.JobStreamRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.sdk.v1.JobEvent.getDefaultInstance()))
              .setSchemaDescriptor(new FunctionServiceMethodDescriptorSupplier("StreamJob"))
              .build();
        }
      }
    }
    return getStreamJobMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.croupier.sdk.v1.CancelJobRequest,
      com.croupier.sdk.v1.StartJobResponse> getCancelJobMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "CancelJob",
      requestType = com.croupier.sdk.v1.CancelJobRequest.class,
      responseType = com.croupier.sdk.v1.StartJobResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.croupier.sdk.v1.CancelJobRequest,
      com.croupier.sdk.v1.StartJobResponse> getCancelJobMethod() {
    io.grpc.MethodDescriptor<com.croupier.sdk.v1.CancelJobRequest, com.croupier.sdk.v1.StartJobResponse> getCancelJobMethod;
    if ((getCancelJobMethod = FunctionServiceGrpc.getCancelJobMethod) == null) {
      synchronized (FunctionServiceGrpc.class) {
        if ((getCancelJobMethod = FunctionServiceGrpc.getCancelJobMethod) == null) {
          FunctionServiceGrpc.getCancelJobMethod = getCancelJobMethod =
              io.grpc.MethodDescriptor.<com.croupier.sdk.v1.CancelJobRequest, com.croupier.sdk.v1.StartJobResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "CancelJob"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.sdk.v1.CancelJobRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.sdk.v1.StartJobResponse.getDefaultInstance()))
              .setSchemaDescriptor(new FunctionServiceMethodDescriptorSupplier("CancelJob"))
              .build();
        }
      }
    }
    return getCancelJobMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static FunctionServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<FunctionServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<FunctionServiceStub>() {
        @java.lang.Override
        public FunctionServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new FunctionServiceStub(channel, callOptions);
        }
      };
    return FunctionServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static FunctionServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<FunctionServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<FunctionServiceBlockingV2Stub>() {
        @java.lang.Override
        public FunctionServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new FunctionServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return FunctionServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static FunctionServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<FunctionServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<FunctionServiceBlockingStub>() {
        @java.lang.Override
        public FunctionServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new FunctionServiceBlockingStub(channel, callOptions);
        }
      };
    return FunctionServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static FunctionServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<FunctionServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<FunctionServiceFutureStub>() {
        @java.lang.Override
        public FunctionServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new FunctionServiceFutureStub(channel, callOptions);
        }
      };
    return FunctionServiceFutureStub.newStub(factory, channel);
  }

  /**
   * <pre>
   * SDK Function Service - Main interface for invoking functions
   * This is what SDK developers will use to call game functions
   * </pre>
   */
  public interface AsyncService {

    /**
     * <pre>
     * Synchronous function invocation
     * </pre>
     */
    default void invoke(com.croupier.sdk.v1.InvokeRequest request,
        io.grpc.stub.StreamObserver<com.croupier.sdk.v1.InvokeResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getInvokeMethod(), responseObserver);
    }

    /**
     * <pre>
     * Asynchronous job starting
     * </pre>
     */
    default void startJob(com.croupier.sdk.v1.InvokeRequest request,
        io.grpc.stub.StreamObserver<com.croupier.sdk.v1.StartJobResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getStartJobMethod(), responseObserver);
    }

    /**
     * <pre>
     * Stream job events for async operations
     * </pre>
     */
    default void streamJob(com.croupier.sdk.v1.JobStreamRequest request,
        io.grpc.stub.StreamObserver<com.croupier.sdk.v1.JobEvent> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getStreamJobMethod(), responseObserver);
    }

    /**
     * <pre>
     * Cancel a running job
     * </pre>
     */
    default void cancelJob(com.croupier.sdk.v1.CancelJobRequest request,
        io.grpc.stub.StreamObserver<com.croupier.sdk.v1.StartJobResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getCancelJobMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service FunctionService.
   * <pre>
   * SDK Function Service - Main interface for invoking functions
   * This is what SDK developers will use to call game functions
   * </pre>
   */
  public static abstract class FunctionServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return FunctionServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service FunctionService.
   * <pre>
   * SDK Function Service - Main interface for invoking functions
   * This is what SDK developers will use to call game functions
   * </pre>
   */
  public static final class FunctionServiceStub
      extends io.grpc.stub.AbstractAsyncStub<FunctionServiceStub> {
    private FunctionServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected FunctionServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new FunctionServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * Synchronous function invocation
     * </pre>
     */
    public void invoke(com.croupier.sdk.v1.InvokeRequest request,
        io.grpc.stub.StreamObserver<com.croupier.sdk.v1.InvokeResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getInvokeMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * Asynchronous job starting
     * </pre>
     */
    public void startJob(com.croupier.sdk.v1.InvokeRequest request,
        io.grpc.stub.StreamObserver<com.croupier.sdk.v1.StartJobResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getStartJobMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * Stream job events for async operations
     * </pre>
     */
    public void streamJob(com.croupier.sdk.v1.JobStreamRequest request,
        io.grpc.stub.StreamObserver<com.croupier.sdk.v1.JobEvent> responseObserver) {
      io.grpc.stub.ClientCalls.asyncServerStreamingCall(
          getChannel().newCall(getStreamJobMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * Cancel a running job
     * </pre>
     */
    public void cancelJob(com.croupier.sdk.v1.CancelJobRequest request,
        io.grpc.stub.StreamObserver<com.croupier.sdk.v1.StartJobResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getCancelJobMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service FunctionService.
   * <pre>
   * SDK Function Service - Main interface for invoking functions
   * This is what SDK developers will use to call game functions
   * </pre>
   */
  public static final class FunctionServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<FunctionServiceBlockingV2Stub> {
    private FunctionServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected FunctionServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new FunctionServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     * <pre>
     * Synchronous function invocation
     * </pre>
     */
    public com.croupier.sdk.v1.InvokeResponse invoke(com.croupier.sdk.v1.InvokeRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getInvokeMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * Asynchronous job starting
     * </pre>
     */
    public com.croupier.sdk.v1.StartJobResponse startJob(com.croupier.sdk.v1.InvokeRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getStartJobMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * Stream job events for async operations
     * </pre>
     */
    @io.grpc.ExperimentalApi("https://github.com/grpc/grpc-java/issues/10918")
    public io.grpc.stub.BlockingClientCall<?, com.croupier.sdk.v1.JobEvent>
        streamJob(com.croupier.sdk.v1.JobStreamRequest request) {
      return io.grpc.stub.ClientCalls.blockingV2ServerStreamingCall(
          getChannel(), getStreamJobMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * Cancel a running job
     * </pre>
     */
    public com.croupier.sdk.v1.StartJobResponse cancelJob(com.croupier.sdk.v1.CancelJobRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getCancelJobMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service FunctionService.
   * <pre>
   * SDK Function Service - Main interface for invoking functions
   * This is what SDK developers will use to call game functions
   * </pre>
   */
  public static final class FunctionServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<FunctionServiceBlockingStub> {
    private FunctionServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected FunctionServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new FunctionServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * Synchronous function invocation
     * </pre>
     */
    public com.croupier.sdk.v1.InvokeResponse invoke(com.croupier.sdk.v1.InvokeRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getInvokeMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * Asynchronous job starting
     * </pre>
     */
    public com.croupier.sdk.v1.StartJobResponse startJob(com.croupier.sdk.v1.InvokeRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getStartJobMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * Stream job events for async operations
     * </pre>
     */
    public java.util.Iterator<com.croupier.sdk.v1.JobEvent> streamJob(
        com.croupier.sdk.v1.JobStreamRequest request) {
      return io.grpc.stub.ClientCalls.blockingServerStreamingCall(
          getChannel(), getStreamJobMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * Cancel a running job
     * </pre>
     */
    public com.croupier.sdk.v1.StartJobResponse cancelJob(com.croupier.sdk.v1.CancelJobRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getCancelJobMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service FunctionService.
   * <pre>
   * SDK Function Service - Main interface for invoking functions
   * This is what SDK developers will use to call game functions
   * </pre>
   */
  public static final class FunctionServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<FunctionServiceFutureStub> {
    private FunctionServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected FunctionServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new FunctionServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * Synchronous function invocation
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.croupier.sdk.v1.InvokeResponse> invoke(
        com.croupier.sdk.v1.InvokeRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getInvokeMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * Asynchronous job starting
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.croupier.sdk.v1.StartJobResponse> startJob(
        com.croupier.sdk.v1.InvokeRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getStartJobMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * Cancel a running job
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.croupier.sdk.v1.StartJobResponse> cancelJob(
        com.croupier.sdk.v1.CancelJobRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getCancelJobMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_INVOKE = 0;
  private static final int METHODID_START_JOB = 1;
  private static final int METHODID_STREAM_JOB = 2;
  private static final int METHODID_CANCEL_JOB = 3;

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
        case METHODID_INVOKE:
          serviceImpl.invoke((com.croupier.sdk.v1.InvokeRequest) request,
              (io.grpc.stub.StreamObserver<com.croupier.sdk.v1.InvokeResponse>) responseObserver);
          break;
        case METHODID_START_JOB:
          serviceImpl.startJob((com.croupier.sdk.v1.InvokeRequest) request,
              (io.grpc.stub.StreamObserver<com.croupier.sdk.v1.StartJobResponse>) responseObserver);
          break;
        case METHODID_STREAM_JOB:
          serviceImpl.streamJob((com.croupier.sdk.v1.JobStreamRequest) request,
              (io.grpc.stub.StreamObserver<com.croupier.sdk.v1.JobEvent>) responseObserver);
          break;
        case METHODID_CANCEL_JOB:
          serviceImpl.cancelJob((com.croupier.sdk.v1.CancelJobRequest) request,
              (io.grpc.stub.StreamObserver<com.croupier.sdk.v1.StartJobResponse>) responseObserver);
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
          getInvokeMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.croupier.sdk.v1.InvokeRequest,
              com.croupier.sdk.v1.InvokeResponse>(
                service, METHODID_INVOKE)))
        .addMethod(
          getStartJobMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.croupier.sdk.v1.InvokeRequest,
              com.croupier.sdk.v1.StartJobResponse>(
                service, METHODID_START_JOB)))
        .addMethod(
          getStreamJobMethod(),
          io.grpc.stub.ServerCalls.asyncServerStreamingCall(
            new MethodHandlers<
              com.croupier.sdk.v1.JobStreamRequest,
              com.croupier.sdk.v1.JobEvent>(
                service, METHODID_STREAM_JOB)))
        .addMethod(
          getCancelJobMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.croupier.sdk.v1.CancelJobRequest,
              com.croupier.sdk.v1.StartJobResponse>(
                service, METHODID_CANCEL_JOB)))
        .build();
  }

  private static abstract class FunctionServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    FunctionServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return com.croupier.sdk.v1.FunctionProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("FunctionService");
    }
  }

  private static final class FunctionServiceFileDescriptorSupplier
      extends FunctionServiceBaseDescriptorSupplier {
    FunctionServiceFileDescriptorSupplier() {}
  }

  private static final class FunctionServiceMethodDescriptorSupplier
      extends FunctionServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    FunctionServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (FunctionServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new FunctionServiceFileDescriptorSupplier())
              .addMethod(getInvokeMethod())
              .addMethod(getStartJobMethod())
              .addMethod(getStreamJobMethod())
              .addMethod(getCancelJobMethod())
              .build();
        }
      }
    }
    return result;
  }
}
