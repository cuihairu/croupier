package com.croupier.server.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * Server Control Service - Internal interface for agent registration and management
 * </pre>
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class ControlServiceGrpc {

  private ControlServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "croupier.server.v1.ControlService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<com.google.protobuf.Empty,
      com.croupier.server.v1.ListFunctionsSummaryResponse> getListFunctionsSummaryMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListFunctionsSummary",
      requestType = com.google.protobuf.Empty.class,
      responseType = com.croupier.server.v1.ListFunctionsSummaryResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.google.protobuf.Empty,
      com.croupier.server.v1.ListFunctionsSummaryResponse> getListFunctionsSummaryMethod() {
    io.grpc.MethodDescriptor<com.google.protobuf.Empty, com.croupier.server.v1.ListFunctionsSummaryResponse> getListFunctionsSummaryMethod;
    if ((getListFunctionsSummaryMethod = ControlServiceGrpc.getListFunctionsSummaryMethod) == null) {
      synchronized (ControlServiceGrpc.class) {
        if ((getListFunctionsSummaryMethod = ControlServiceGrpc.getListFunctionsSummaryMethod) == null) {
          ControlServiceGrpc.getListFunctionsSummaryMethod = getListFunctionsSummaryMethod =
              io.grpc.MethodDescriptor.<com.google.protobuf.Empty, com.croupier.server.v1.ListFunctionsSummaryResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListFunctionsSummary"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.google.protobuf.Empty.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.server.v1.ListFunctionsSummaryResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ControlServiceMethodDescriptorSupplier("ListFunctionsSummary"))
              .build();
        }
      }
    }
    return getListFunctionsSummaryMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.croupier.server.v1.RegisterRequest,
      com.croupier.server.v1.RegisterResponse> getRegisterMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "Register",
      requestType = com.croupier.server.v1.RegisterRequest.class,
      responseType = com.croupier.server.v1.RegisterResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.croupier.server.v1.RegisterRequest,
      com.croupier.server.v1.RegisterResponse> getRegisterMethod() {
    io.grpc.MethodDescriptor<com.croupier.server.v1.RegisterRequest, com.croupier.server.v1.RegisterResponse> getRegisterMethod;
    if ((getRegisterMethod = ControlServiceGrpc.getRegisterMethod) == null) {
      synchronized (ControlServiceGrpc.class) {
        if ((getRegisterMethod = ControlServiceGrpc.getRegisterMethod) == null) {
          ControlServiceGrpc.getRegisterMethod = getRegisterMethod =
              io.grpc.MethodDescriptor.<com.croupier.server.v1.RegisterRequest, com.croupier.server.v1.RegisterResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "Register"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.server.v1.RegisterRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.server.v1.RegisterResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ControlServiceMethodDescriptorSupplier("Register"))
              .build();
        }
      }
    }
    return getRegisterMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.croupier.server.v1.HeartbeatRequest,
      com.croupier.server.v1.HeartbeatResponse> getHeartbeatMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "Heartbeat",
      requestType = com.croupier.server.v1.HeartbeatRequest.class,
      responseType = com.croupier.server.v1.HeartbeatResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.croupier.server.v1.HeartbeatRequest,
      com.croupier.server.v1.HeartbeatResponse> getHeartbeatMethod() {
    io.grpc.MethodDescriptor<com.croupier.server.v1.HeartbeatRequest, com.croupier.server.v1.HeartbeatResponse> getHeartbeatMethod;
    if ((getHeartbeatMethod = ControlServiceGrpc.getHeartbeatMethod) == null) {
      synchronized (ControlServiceGrpc.class) {
        if ((getHeartbeatMethod = ControlServiceGrpc.getHeartbeatMethod) == null) {
          ControlServiceGrpc.getHeartbeatMethod = getHeartbeatMethod =
              io.grpc.MethodDescriptor.<com.croupier.server.v1.HeartbeatRequest, com.croupier.server.v1.HeartbeatResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "Heartbeat"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.server.v1.HeartbeatRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.server.v1.HeartbeatResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ControlServiceMethodDescriptorSupplier("Heartbeat"))
              .build();
        }
      }
    }
    return getHeartbeatMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.croupier.server.v1.RegisterCapabilitiesRequest,
      com.croupier.server.v1.RegisterCapabilitiesResponse> getRegisterCapabilitiesMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "RegisterCapabilities",
      requestType = com.croupier.server.v1.RegisterCapabilitiesRequest.class,
      responseType = com.croupier.server.v1.RegisterCapabilitiesResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.croupier.server.v1.RegisterCapabilitiesRequest,
      com.croupier.server.v1.RegisterCapabilitiesResponse> getRegisterCapabilitiesMethod() {
    io.grpc.MethodDescriptor<com.croupier.server.v1.RegisterCapabilitiesRequest, com.croupier.server.v1.RegisterCapabilitiesResponse> getRegisterCapabilitiesMethod;
    if ((getRegisterCapabilitiesMethod = ControlServiceGrpc.getRegisterCapabilitiesMethod) == null) {
      synchronized (ControlServiceGrpc.class) {
        if ((getRegisterCapabilitiesMethod = ControlServiceGrpc.getRegisterCapabilitiesMethod) == null) {
          ControlServiceGrpc.getRegisterCapabilitiesMethod = getRegisterCapabilitiesMethod =
              io.grpc.MethodDescriptor.<com.croupier.server.v1.RegisterCapabilitiesRequest, com.croupier.server.v1.RegisterCapabilitiesResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "RegisterCapabilities"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.server.v1.RegisterCapabilitiesRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.server.v1.RegisterCapabilitiesResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ControlServiceMethodDescriptorSupplier("RegisterCapabilities"))
              .build();
        }
      }
    }
    return getRegisterCapabilitiesMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static ControlServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ControlServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ControlServiceStub>() {
        @java.lang.Override
        public ControlServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ControlServiceStub(channel, callOptions);
        }
      };
    return ControlServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static ControlServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ControlServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ControlServiceBlockingV2Stub>() {
        @java.lang.Override
        public ControlServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ControlServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return ControlServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static ControlServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ControlServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ControlServiceBlockingStub>() {
        @java.lang.Override
        public ControlServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ControlServiceBlockingStub(channel, callOptions);
        }
      };
    return ControlServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static ControlServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ControlServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ControlServiceFutureStub>() {
        @java.lang.Override
        public ControlServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ControlServiceFutureStub(channel, callOptions);
        }
      };
    return ControlServiceFutureStub.newStub(factory, channel);
  }

  /**
   * <pre>
   * Server Control Service - Internal interface for agent registration and management
   * </pre>
   */
  public interface AsyncService {

    /**
     * <pre>
     * Summarized function catalog with UI/RBAC metadata (for dashboard)
     * </pre>
     */
    default void listFunctionsSummary(com.google.protobuf.Empty request,
        io.grpc.stub.StreamObserver<com.croupier.server.v1.ListFunctionsSummaryResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListFunctionsSummaryMethod(), responseObserver);
    }

    /**
     * <pre>
     * Agent registration with server
     * </pre>
     */
    default void register(com.croupier.server.v1.RegisterRequest request,
        io.grpc.stub.StreamObserver<com.croupier.server.v1.RegisterResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getRegisterMethod(), responseObserver);
    }

    /**
     * <pre>
     * Agent heartbeat
     * </pre>
     */
    default void heartbeat(com.croupier.server.v1.HeartbeatRequest request,
        io.grpc.stub.StreamObserver<com.croupier.server.v1.HeartbeatResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getHeartbeatMethod(), responseObserver);
    }

    /**
     * <pre>
     * Provider capabilities registration
     * </pre>
     */
    default void registerCapabilities(com.croupier.server.v1.RegisterCapabilitiesRequest request,
        io.grpc.stub.StreamObserver<com.croupier.server.v1.RegisterCapabilitiesResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getRegisterCapabilitiesMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service ControlService.
   * <pre>
   * Server Control Service - Internal interface for agent registration and management
   * </pre>
   */
  public static abstract class ControlServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return ControlServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service ControlService.
   * <pre>
   * Server Control Service - Internal interface for agent registration and management
   * </pre>
   */
  public static final class ControlServiceStub
      extends io.grpc.stub.AbstractAsyncStub<ControlServiceStub> {
    private ControlServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ControlServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ControlServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * Summarized function catalog with UI/RBAC metadata (for dashboard)
     * </pre>
     */
    public void listFunctionsSummary(com.google.protobuf.Empty request,
        io.grpc.stub.StreamObserver<com.croupier.server.v1.ListFunctionsSummaryResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListFunctionsSummaryMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * Agent registration with server
     * </pre>
     */
    public void register(com.croupier.server.v1.RegisterRequest request,
        io.grpc.stub.StreamObserver<com.croupier.server.v1.RegisterResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getRegisterMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * Agent heartbeat
     * </pre>
     */
    public void heartbeat(com.croupier.server.v1.HeartbeatRequest request,
        io.grpc.stub.StreamObserver<com.croupier.server.v1.HeartbeatResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getHeartbeatMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * Provider capabilities registration
     * </pre>
     */
    public void registerCapabilities(com.croupier.server.v1.RegisterCapabilitiesRequest request,
        io.grpc.stub.StreamObserver<com.croupier.server.v1.RegisterCapabilitiesResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getRegisterCapabilitiesMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service ControlService.
   * <pre>
   * Server Control Service - Internal interface for agent registration and management
   * </pre>
   */
  public static final class ControlServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<ControlServiceBlockingV2Stub> {
    private ControlServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ControlServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ControlServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     * <pre>
     * Summarized function catalog with UI/RBAC metadata (for dashboard)
     * </pre>
     */
    public com.croupier.server.v1.ListFunctionsSummaryResponse listFunctionsSummary(com.google.protobuf.Empty request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getListFunctionsSummaryMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * Agent registration with server
     * </pre>
     */
    public com.croupier.server.v1.RegisterResponse register(com.croupier.server.v1.RegisterRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getRegisterMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * Agent heartbeat
     * </pre>
     */
    public com.croupier.server.v1.HeartbeatResponse heartbeat(com.croupier.server.v1.HeartbeatRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getHeartbeatMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * Provider capabilities registration
     * </pre>
     */
    public com.croupier.server.v1.RegisterCapabilitiesResponse registerCapabilities(com.croupier.server.v1.RegisterCapabilitiesRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getRegisterCapabilitiesMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service ControlService.
   * <pre>
   * Server Control Service - Internal interface for agent registration and management
   * </pre>
   */
  public static final class ControlServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<ControlServiceBlockingStub> {
    private ControlServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ControlServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ControlServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * Summarized function catalog with UI/RBAC metadata (for dashboard)
     * </pre>
     */
    public com.croupier.server.v1.ListFunctionsSummaryResponse listFunctionsSummary(com.google.protobuf.Empty request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListFunctionsSummaryMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * Agent registration with server
     * </pre>
     */
    public com.croupier.server.v1.RegisterResponse register(com.croupier.server.v1.RegisterRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getRegisterMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * Agent heartbeat
     * </pre>
     */
    public com.croupier.server.v1.HeartbeatResponse heartbeat(com.croupier.server.v1.HeartbeatRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getHeartbeatMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * Provider capabilities registration
     * </pre>
     */
    public com.croupier.server.v1.RegisterCapabilitiesResponse registerCapabilities(com.croupier.server.v1.RegisterCapabilitiesRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getRegisterCapabilitiesMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service ControlService.
   * <pre>
   * Server Control Service - Internal interface for agent registration and management
   * </pre>
   */
  public static final class ControlServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<ControlServiceFutureStub> {
    private ControlServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ControlServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ControlServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * Summarized function catalog with UI/RBAC metadata (for dashboard)
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.croupier.server.v1.ListFunctionsSummaryResponse> listFunctionsSummary(
        com.google.protobuf.Empty request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListFunctionsSummaryMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * Agent registration with server
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.croupier.server.v1.RegisterResponse> register(
        com.croupier.server.v1.RegisterRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getRegisterMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * Agent heartbeat
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.croupier.server.v1.HeartbeatResponse> heartbeat(
        com.croupier.server.v1.HeartbeatRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getHeartbeatMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * Provider capabilities registration
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.croupier.server.v1.RegisterCapabilitiesResponse> registerCapabilities(
        com.croupier.server.v1.RegisterCapabilitiesRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getRegisterCapabilitiesMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_LIST_FUNCTIONS_SUMMARY = 0;
  private static final int METHODID_REGISTER = 1;
  private static final int METHODID_HEARTBEAT = 2;
  private static final int METHODID_REGISTER_CAPABILITIES = 3;

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
        case METHODID_LIST_FUNCTIONS_SUMMARY:
          serviceImpl.listFunctionsSummary((com.google.protobuf.Empty) request,
              (io.grpc.stub.StreamObserver<com.croupier.server.v1.ListFunctionsSummaryResponse>) responseObserver);
          break;
        case METHODID_REGISTER:
          serviceImpl.register((com.croupier.server.v1.RegisterRequest) request,
              (io.grpc.stub.StreamObserver<com.croupier.server.v1.RegisterResponse>) responseObserver);
          break;
        case METHODID_HEARTBEAT:
          serviceImpl.heartbeat((com.croupier.server.v1.HeartbeatRequest) request,
              (io.grpc.stub.StreamObserver<com.croupier.server.v1.HeartbeatResponse>) responseObserver);
          break;
        case METHODID_REGISTER_CAPABILITIES:
          serviceImpl.registerCapabilities((com.croupier.server.v1.RegisterCapabilitiesRequest) request,
              (io.grpc.stub.StreamObserver<com.croupier.server.v1.RegisterCapabilitiesResponse>) responseObserver);
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
          getListFunctionsSummaryMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.google.protobuf.Empty,
              com.croupier.server.v1.ListFunctionsSummaryResponse>(
                service, METHODID_LIST_FUNCTIONS_SUMMARY)))
        .addMethod(
          getRegisterMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.croupier.server.v1.RegisterRequest,
              com.croupier.server.v1.RegisterResponse>(
                service, METHODID_REGISTER)))
        .addMethod(
          getHeartbeatMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.croupier.server.v1.HeartbeatRequest,
              com.croupier.server.v1.HeartbeatResponse>(
                service, METHODID_HEARTBEAT)))
        .addMethod(
          getRegisterCapabilitiesMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.croupier.server.v1.RegisterCapabilitiesRequest,
              com.croupier.server.v1.RegisterCapabilitiesResponse>(
                service, METHODID_REGISTER_CAPABILITIES)))
        .build();
  }

  private static abstract class ControlServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    ControlServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return com.croupier.server.v1.ControlProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("ControlService");
    }
  }

  private static final class ControlServiceFileDescriptorSupplier
      extends ControlServiceBaseDescriptorSupplier {
    ControlServiceFileDescriptorSupplier() {}
  }

  private static final class ControlServiceMethodDescriptorSupplier
      extends ControlServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    ControlServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (ControlServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new ControlServiceFileDescriptorSupplier())
              .addMethod(getListFunctionsSummaryMethod())
              .addMethod(getRegisterMethod())
              .addMethod(getHeartbeatMethod())
              .addMethod(getRegisterCapabilitiesMethod())
              .build();
        }
      }
    }
    return result;
  }
}
