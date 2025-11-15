package com.croupier.agent.local.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class LocalControlServiceGrpc {

  private LocalControlServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "croupier.agent.local.v1.LocalControlService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<com.croupier.agent.local.v1.RegisterLocalRequest,
      com.croupier.agent.local.v1.RegisterLocalResponse> getRegisterLocalMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "RegisterLocal",
      requestType = com.croupier.agent.local.v1.RegisterLocalRequest.class,
      responseType = com.croupier.agent.local.v1.RegisterLocalResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.croupier.agent.local.v1.RegisterLocalRequest,
      com.croupier.agent.local.v1.RegisterLocalResponse> getRegisterLocalMethod() {
    io.grpc.MethodDescriptor<com.croupier.agent.local.v1.RegisterLocalRequest, com.croupier.agent.local.v1.RegisterLocalResponse> getRegisterLocalMethod;
    if ((getRegisterLocalMethod = LocalControlServiceGrpc.getRegisterLocalMethod) == null) {
      synchronized (LocalControlServiceGrpc.class) {
        if ((getRegisterLocalMethod = LocalControlServiceGrpc.getRegisterLocalMethod) == null) {
          LocalControlServiceGrpc.getRegisterLocalMethod = getRegisterLocalMethod =
              io.grpc.MethodDescriptor.<com.croupier.agent.local.v1.RegisterLocalRequest, com.croupier.agent.local.v1.RegisterLocalResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "RegisterLocal"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.agent.local.v1.RegisterLocalRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.agent.local.v1.RegisterLocalResponse.getDefaultInstance()))
              .setSchemaDescriptor(new LocalControlServiceMethodDescriptorSupplier("RegisterLocal"))
              .build();
        }
      }
    }
    return getRegisterLocalMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.croupier.agent.local.v1.HeartbeatRequest,
      com.croupier.agent.local.v1.HeartbeatResponse> getHeartbeatMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "Heartbeat",
      requestType = com.croupier.agent.local.v1.HeartbeatRequest.class,
      responseType = com.croupier.agent.local.v1.HeartbeatResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.croupier.agent.local.v1.HeartbeatRequest,
      com.croupier.agent.local.v1.HeartbeatResponse> getHeartbeatMethod() {
    io.grpc.MethodDescriptor<com.croupier.agent.local.v1.HeartbeatRequest, com.croupier.agent.local.v1.HeartbeatResponse> getHeartbeatMethod;
    if ((getHeartbeatMethod = LocalControlServiceGrpc.getHeartbeatMethod) == null) {
      synchronized (LocalControlServiceGrpc.class) {
        if ((getHeartbeatMethod = LocalControlServiceGrpc.getHeartbeatMethod) == null) {
          LocalControlServiceGrpc.getHeartbeatMethod = getHeartbeatMethod =
              io.grpc.MethodDescriptor.<com.croupier.agent.local.v1.HeartbeatRequest, com.croupier.agent.local.v1.HeartbeatResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "Heartbeat"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.agent.local.v1.HeartbeatRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.agent.local.v1.HeartbeatResponse.getDefaultInstance()))
              .setSchemaDescriptor(new LocalControlServiceMethodDescriptorSupplier("Heartbeat"))
              .build();
        }
      }
    }
    return getHeartbeatMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.croupier.agent.local.v1.ListLocalRequest,
      com.croupier.agent.local.v1.ListLocalResponse> getListLocalMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListLocal",
      requestType = com.croupier.agent.local.v1.ListLocalRequest.class,
      responseType = com.croupier.agent.local.v1.ListLocalResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.croupier.agent.local.v1.ListLocalRequest,
      com.croupier.agent.local.v1.ListLocalResponse> getListLocalMethod() {
    io.grpc.MethodDescriptor<com.croupier.agent.local.v1.ListLocalRequest, com.croupier.agent.local.v1.ListLocalResponse> getListLocalMethod;
    if ((getListLocalMethod = LocalControlServiceGrpc.getListLocalMethod) == null) {
      synchronized (LocalControlServiceGrpc.class) {
        if ((getListLocalMethod = LocalControlServiceGrpc.getListLocalMethod) == null) {
          LocalControlServiceGrpc.getListLocalMethod = getListLocalMethod =
              io.grpc.MethodDescriptor.<com.croupier.agent.local.v1.ListLocalRequest, com.croupier.agent.local.v1.ListLocalResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListLocal"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.agent.local.v1.ListLocalRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.agent.local.v1.ListLocalResponse.getDefaultInstance()))
              .setSchemaDescriptor(new LocalControlServiceMethodDescriptorSupplier("ListLocal"))
              .build();
        }
      }
    }
    return getListLocalMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.croupier.agent.local.v1.GetJobResultRequest,
      com.croupier.agent.local.v1.GetJobResultResponse> getGetJobResultMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetJobResult",
      requestType = com.croupier.agent.local.v1.GetJobResultRequest.class,
      responseType = com.croupier.agent.local.v1.GetJobResultResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.croupier.agent.local.v1.GetJobResultRequest,
      com.croupier.agent.local.v1.GetJobResultResponse> getGetJobResultMethod() {
    io.grpc.MethodDescriptor<com.croupier.agent.local.v1.GetJobResultRequest, com.croupier.agent.local.v1.GetJobResultResponse> getGetJobResultMethod;
    if ((getGetJobResultMethod = LocalControlServiceGrpc.getGetJobResultMethod) == null) {
      synchronized (LocalControlServiceGrpc.class) {
        if ((getGetJobResultMethod = LocalControlServiceGrpc.getGetJobResultMethod) == null) {
          LocalControlServiceGrpc.getGetJobResultMethod = getGetJobResultMethod =
              io.grpc.MethodDescriptor.<com.croupier.agent.local.v1.GetJobResultRequest, com.croupier.agent.local.v1.GetJobResultResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetJobResult"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.agent.local.v1.GetJobResultRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.agent.local.v1.GetJobResultResponse.getDefaultInstance()))
              .setSchemaDescriptor(new LocalControlServiceMethodDescriptorSupplier("GetJobResult"))
              .build();
        }
      }
    }
    return getGetJobResultMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static LocalControlServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<LocalControlServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<LocalControlServiceStub>() {
        @java.lang.Override
        public LocalControlServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new LocalControlServiceStub(channel, callOptions);
        }
      };
    return LocalControlServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static LocalControlServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<LocalControlServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<LocalControlServiceBlockingV2Stub>() {
        @java.lang.Override
        public LocalControlServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new LocalControlServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return LocalControlServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static LocalControlServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<LocalControlServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<LocalControlServiceBlockingStub>() {
        @java.lang.Override
        public LocalControlServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new LocalControlServiceBlockingStub(channel, callOptions);
        }
      };
    return LocalControlServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static LocalControlServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<LocalControlServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<LocalControlServiceFutureStub>() {
        @java.lang.Override
        public LocalControlServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new LocalControlServiceFutureStub(channel, callOptions);
        }
      };
    return LocalControlServiceFutureStub.newStub(factory, channel);
  }

  /**
   */
  public interface AsyncService {

    /**
     */
    default void registerLocal(com.croupier.agent.local.v1.RegisterLocalRequest request,
        io.grpc.stub.StreamObserver<com.croupier.agent.local.v1.RegisterLocalResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getRegisterLocalMethod(), responseObserver);
    }

    /**
     */
    default void heartbeat(com.croupier.agent.local.v1.HeartbeatRequest request,
        io.grpc.stub.StreamObserver<com.croupier.agent.local.v1.HeartbeatResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getHeartbeatMethod(), responseObserver);
    }

    /**
     */
    default void listLocal(com.croupier.agent.local.v1.ListLocalRequest request,
        io.grpc.stub.StreamObserver<com.croupier.agent.local.v1.ListLocalResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListLocalMethod(), responseObserver);
    }

    /**
     */
    default void getJobResult(com.croupier.agent.local.v1.GetJobResultRequest request,
        io.grpc.stub.StreamObserver<com.croupier.agent.local.v1.GetJobResultResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetJobResultMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service LocalControlService.
   */
  public static abstract class LocalControlServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return LocalControlServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service LocalControlService.
   */
  public static final class LocalControlServiceStub
      extends io.grpc.stub.AbstractAsyncStub<LocalControlServiceStub> {
    private LocalControlServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected LocalControlServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new LocalControlServiceStub(channel, callOptions);
    }

    /**
     */
    public void registerLocal(com.croupier.agent.local.v1.RegisterLocalRequest request,
        io.grpc.stub.StreamObserver<com.croupier.agent.local.v1.RegisterLocalResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getRegisterLocalMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void heartbeat(com.croupier.agent.local.v1.HeartbeatRequest request,
        io.grpc.stub.StreamObserver<com.croupier.agent.local.v1.HeartbeatResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getHeartbeatMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void listLocal(com.croupier.agent.local.v1.ListLocalRequest request,
        io.grpc.stub.StreamObserver<com.croupier.agent.local.v1.ListLocalResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListLocalMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     */
    public void getJobResult(com.croupier.agent.local.v1.GetJobResultRequest request,
        io.grpc.stub.StreamObserver<com.croupier.agent.local.v1.GetJobResultResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetJobResultMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service LocalControlService.
   */
  public static final class LocalControlServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<LocalControlServiceBlockingV2Stub> {
    private LocalControlServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected LocalControlServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new LocalControlServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     */
    public com.croupier.agent.local.v1.RegisterLocalResponse registerLocal(com.croupier.agent.local.v1.RegisterLocalRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getRegisterLocalMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.croupier.agent.local.v1.HeartbeatResponse heartbeat(com.croupier.agent.local.v1.HeartbeatRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getHeartbeatMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.croupier.agent.local.v1.ListLocalResponse listLocal(com.croupier.agent.local.v1.ListLocalRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getListLocalMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.croupier.agent.local.v1.GetJobResultResponse getJobResult(com.croupier.agent.local.v1.GetJobResultRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetJobResultMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service LocalControlService.
   */
  public static final class LocalControlServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<LocalControlServiceBlockingStub> {
    private LocalControlServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected LocalControlServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new LocalControlServiceBlockingStub(channel, callOptions);
    }

    /**
     */
    public com.croupier.agent.local.v1.RegisterLocalResponse registerLocal(com.croupier.agent.local.v1.RegisterLocalRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getRegisterLocalMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.croupier.agent.local.v1.HeartbeatResponse heartbeat(com.croupier.agent.local.v1.HeartbeatRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getHeartbeatMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.croupier.agent.local.v1.ListLocalResponse listLocal(com.croupier.agent.local.v1.ListLocalRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListLocalMethod(), getCallOptions(), request);
    }

    /**
     */
    public com.croupier.agent.local.v1.GetJobResultResponse getJobResult(com.croupier.agent.local.v1.GetJobResultRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetJobResultMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service LocalControlService.
   */
  public static final class LocalControlServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<LocalControlServiceFutureStub> {
    private LocalControlServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected LocalControlServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new LocalControlServiceFutureStub(channel, callOptions);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.croupier.agent.local.v1.RegisterLocalResponse> registerLocal(
        com.croupier.agent.local.v1.RegisterLocalRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getRegisterLocalMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.croupier.agent.local.v1.HeartbeatResponse> heartbeat(
        com.croupier.agent.local.v1.HeartbeatRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getHeartbeatMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.croupier.agent.local.v1.ListLocalResponse> listLocal(
        com.croupier.agent.local.v1.ListLocalRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListLocalMethod(), getCallOptions()), request);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.croupier.agent.local.v1.GetJobResultResponse> getJobResult(
        com.croupier.agent.local.v1.GetJobResultRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetJobResultMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_REGISTER_LOCAL = 0;
  private static final int METHODID_HEARTBEAT = 1;
  private static final int METHODID_LIST_LOCAL = 2;
  private static final int METHODID_GET_JOB_RESULT = 3;

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
        case METHODID_REGISTER_LOCAL:
          serviceImpl.registerLocal((com.croupier.agent.local.v1.RegisterLocalRequest) request,
              (io.grpc.stub.StreamObserver<com.croupier.agent.local.v1.RegisterLocalResponse>) responseObserver);
          break;
        case METHODID_HEARTBEAT:
          serviceImpl.heartbeat((com.croupier.agent.local.v1.HeartbeatRequest) request,
              (io.grpc.stub.StreamObserver<com.croupier.agent.local.v1.HeartbeatResponse>) responseObserver);
          break;
        case METHODID_LIST_LOCAL:
          serviceImpl.listLocal((com.croupier.agent.local.v1.ListLocalRequest) request,
              (io.grpc.stub.StreamObserver<com.croupier.agent.local.v1.ListLocalResponse>) responseObserver);
          break;
        case METHODID_GET_JOB_RESULT:
          serviceImpl.getJobResult((com.croupier.agent.local.v1.GetJobResultRequest) request,
              (io.grpc.stub.StreamObserver<com.croupier.agent.local.v1.GetJobResultResponse>) responseObserver);
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
          getRegisterLocalMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.croupier.agent.local.v1.RegisterLocalRequest,
              com.croupier.agent.local.v1.RegisterLocalResponse>(
                service, METHODID_REGISTER_LOCAL)))
        .addMethod(
          getHeartbeatMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.croupier.agent.local.v1.HeartbeatRequest,
              com.croupier.agent.local.v1.HeartbeatResponse>(
                service, METHODID_HEARTBEAT)))
        .addMethod(
          getListLocalMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.croupier.agent.local.v1.ListLocalRequest,
              com.croupier.agent.local.v1.ListLocalResponse>(
                service, METHODID_LIST_LOCAL)))
        .addMethod(
          getGetJobResultMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.croupier.agent.local.v1.GetJobResultRequest,
              com.croupier.agent.local.v1.GetJobResultResponse>(
                service, METHODID_GET_JOB_RESULT)))
        .build();
  }

  private static abstract class LocalControlServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    LocalControlServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return com.croupier.agent.local.v1.LocalProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("LocalControlService");
    }
  }

  private static final class LocalControlServiceFileDescriptorSupplier
      extends LocalControlServiceBaseDescriptorSupplier {
    LocalControlServiceFileDescriptorSupplier() {}
  }

  private static final class LocalControlServiceMethodDescriptorSupplier
      extends LocalControlServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    LocalControlServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (LocalControlServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new LocalControlServiceFileDescriptorSupplier())
              .addMethod(getRegisterLocalMethod())
              .addMethod(getHeartbeatMethod())
              .addMethod(getListLocalMethod())
              .addMethod(getGetJobResultMethod())
              .build();
        }
      }
    }
    return result;
  }
}
