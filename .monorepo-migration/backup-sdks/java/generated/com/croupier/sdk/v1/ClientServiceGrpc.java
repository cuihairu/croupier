package com.croupier.sdk.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 * <pre>
 * SDK Client Service - Interface for registering game functions with Agent
 * This is what SDK clients use to register themselves and their functions
 * </pre>
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class ClientServiceGrpc {

  private ClientServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "croupier.sdk.v1.ClientService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<com.croupier.sdk.v1.RegisterClientRequest,
      com.croupier.sdk.v1.RegisterClientResponse> getRegisterClientMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "RegisterClient",
      requestType = com.croupier.sdk.v1.RegisterClientRequest.class,
      responseType = com.croupier.sdk.v1.RegisterClientResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.croupier.sdk.v1.RegisterClientRequest,
      com.croupier.sdk.v1.RegisterClientResponse> getRegisterClientMethod() {
    io.grpc.MethodDescriptor<com.croupier.sdk.v1.RegisterClientRequest, com.croupier.sdk.v1.RegisterClientResponse> getRegisterClientMethod;
    if ((getRegisterClientMethod = ClientServiceGrpc.getRegisterClientMethod) == null) {
      synchronized (ClientServiceGrpc.class) {
        if ((getRegisterClientMethod = ClientServiceGrpc.getRegisterClientMethod) == null) {
          ClientServiceGrpc.getRegisterClientMethod = getRegisterClientMethod =
              io.grpc.MethodDescriptor.<com.croupier.sdk.v1.RegisterClientRequest, com.croupier.sdk.v1.RegisterClientResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "RegisterClient"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.sdk.v1.RegisterClientRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.sdk.v1.RegisterClientResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ClientServiceMethodDescriptorSupplier("RegisterClient"))
              .build();
        }
      }
    }
    return getRegisterClientMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.croupier.sdk.v1.HeartbeatRequest,
      com.croupier.sdk.v1.HeartbeatResponse> getHeartbeatMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "Heartbeat",
      requestType = com.croupier.sdk.v1.HeartbeatRequest.class,
      responseType = com.croupier.sdk.v1.HeartbeatResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.croupier.sdk.v1.HeartbeatRequest,
      com.croupier.sdk.v1.HeartbeatResponse> getHeartbeatMethod() {
    io.grpc.MethodDescriptor<com.croupier.sdk.v1.HeartbeatRequest, com.croupier.sdk.v1.HeartbeatResponse> getHeartbeatMethod;
    if ((getHeartbeatMethod = ClientServiceGrpc.getHeartbeatMethod) == null) {
      synchronized (ClientServiceGrpc.class) {
        if ((getHeartbeatMethod = ClientServiceGrpc.getHeartbeatMethod) == null) {
          ClientServiceGrpc.getHeartbeatMethod = getHeartbeatMethod =
              io.grpc.MethodDescriptor.<com.croupier.sdk.v1.HeartbeatRequest, com.croupier.sdk.v1.HeartbeatResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "Heartbeat"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.sdk.v1.HeartbeatRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.sdk.v1.HeartbeatResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ClientServiceMethodDescriptorSupplier("Heartbeat"))
              .build();
        }
      }
    }
    return getHeartbeatMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.croupier.sdk.v1.ListClientsRequest,
      com.croupier.sdk.v1.ListClientsResponse> getListClientsMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "ListClients",
      requestType = com.croupier.sdk.v1.ListClientsRequest.class,
      responseType = com.croupier.sdk.v1.ListClientsResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.croupier.sdk.v1.ListClientsRequest,
      com.croupier.sdk.v1.ListClientsResponse> getListClientsMethod() {
    io.grpc.MethodDescriptor<com.croupier.sdk.v1.ListClientsRequest, com.croupier.sdk.v1.ListClientsResponse> getListClientsMethod;
    if ((getListClientsMethod = ClientServiceGrpc.getListClientsMethod) == null) {
      synchronized (ClientServiceGrpc.class) {
        if ((getListClientsMethod = ClientServiceGrpc.getListClientsMethod) == null) {
          ClientServiceGrpc.getListClientsMethod = getListClientsMethod =
              io.grpc.MethodDescriptor.<com.croupier.sdk.v1.ListClientsRequest, com.croupier.sdk.v1.ListClientsResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "ListClients"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.sdk.v1.ListClientsRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.sdk.v1.ListClientsResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ClientServiceMethodDescriptorSupplier("ListClients"))
              .build();
        }
      }
    }
    return getListClientsMethod;
  }

  private static volatile io.grpc.MethodDescriptor<com.croupier.sdk.v1.GetJobResultRequest,
      com.croupier.sdk.v1.GetJobResultResponse> getGetJobResultMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetJobResult",
      requestType = com.croupier.sdk.v1.GetJobResultRequest.class,
      responseType = com.croupier.sdk.v1.GetJobResultResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.croupier.sdk.v1.GetJobResultRequest,
      com.croupier.sdk.v1.GetJobResultResponse> getGetJobResultMethod() {
    io.grpc.MethodDescriptor<com.croupier.sdk.v1.GetJobResultRequest, com.croupier.sdk.v1.GetJobResultResponse> getGetJobResultMethod;
    if ((getGetJobResultMethod = ClientServiceGrpc.getGetJobResultMethod) == null) {
      synchronized (ClientServiceGrpc.class) {
        if ((getGetJobResultMethod = ClientServiceGrpc.getGetJobResultMethod) == null) {
          ClientServiceGrpc.getGetJobResultMethod = getGetJobResultMethod =
              io.grpc.MethodDescriptor.<com.croupier.sdk.v1.GetJobResultRequest, com.croupier.sdk.v1.GetJobResultResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetJobResult"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.sdk.v1.GetJobResultRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.sdk.v1.GetJobResultResponse.getDefaultInstance()))
              .setSchemaDescriptor(new ClientServiceMethodDescriptorSupplier("GetJobResult"))
              .build();
        }
      }
    }
    return getGetJobResultMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static ClientServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ClientServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ClientServiceStub>() {
        @java.lang.Override
        public ClientServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ClientServiceStub(channel, callOptions);
        }
      };
    return ClientServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static ClientServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ClientServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ClientServiceBlockingV2Stub>() {
        @java.lang.Override
        public ClientServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ClientServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return ClientServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static ClientServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ClientServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ClientServiceBlockingStub>() {
        @java.lang.Override
        public ClientServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ClientServiceBlockingStub(channel, callOptions);
        }
      };
    return ClientServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static ClientServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<ClientServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<ClientServiceFutureStub>() {
        @java.lang.Override
        public ClientServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new ClientServiceFutureStub(channel, callOptions);
        }
      };
    return ClientServiceFutureStub.newStub(factory, channel);
  }

  /**
   * <pre>
   * SDK Client Service - Interface for registering game functions with Agent
   * This is what SDK clients use to register themselves and their functions
   * </pre>
   */
  public interface AsyncService {

    /**
     * <pre>
     * Register this client and its functions with the Agent
     * </pre>
     */
    default void registerClient(com.croupier.sdk.v1.RegisterClientRequest request,
        io.grpc.stub.StreamObserver<com.croupier.sdk.v1.RegisterClientResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getRegisterClientMethod(), responseObserver);
    }

    /**
     * <pre>
     * Send periodic heartbeats to maintain session
     * </pre>
     */
    default void heartbeat(com.croupier.sdk.v1.HeartbeatRequest request,
        io.grpc.stub.StreamObserver<com.croupier.sdk.v1.HeartbeatResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getHeartbeatMethod(), responseObserver);
    }

    /**
     * <pre>
     * List all registered clients and their functions
     * </pre>
     */
    default void listClients(com.croupier.sdk.v1.ListClientsRequest request,
        io.grpc.stub.StreamObserver<com.croupier.sdk.v1.ListClientsResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getListClientsMethod(), responseObserver);
    }

    /**
     * <pre>
     * Get job result (best-effort)
     * </pre>
     */
    default void getJobResult(com.croupier.sdk.v1.GetJobResultRequest request,
        io.grpc.stub.StreamObserver<com.croupier.sdk.v1.GetJobResultResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetJobResultMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service ClientService.
   * <pre>
   * SDK Client Service - Interface for registering game functions with Agent
   * This is what SDK clients use to register themselves and their functions
   * </pre>
   */
  public static abstract class ClientServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return ClientServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service ClientService.
   * <pre>
   * SDK Client Service - Interface for registering game functions with Agent
   * This is what SDK clients use to register themselves and their functions
   * </pre>
   */
  public static final class ClientServiceStub
      extends io.grpc.stub.AbstractAsyncStub<ClientServiceStub> {
    private ClientServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ClientServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ClientServiceStub(channel, callOptions);
    }

    /**
     * <pre>
     * Register this client and its functions with the Agent
     * </pre>
     */
    public void registerClient(com.croupier.sdk.v1.RegisterClientRequest request,
        io.grpc.stub.StreamObserver<com.croupier.sdk.v1.RegisterClientResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getRegisterClientMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * Send periodic heartbeats to maintain session
     * </pre>
     */
    public void heartbeat(com.croupier.sdk.v1.HeartbeatRequest request,
        io.grpc.stub.StreamObserver<com.croupier.sdk.v1.HeartbeatResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getHeartbeatMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * List all registered clients and their functions
     * </pre>
     */
    public void listClients(com.croupier.sdk.v1.ListClientsRequest request,
        io.grpc.stub.StreamObserver<com.croupier.sdk.v1.ListClientsResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getListClientsMethod(), getCallOptions()), request, responseObserver);
    }

    /**
     * <pre>
     * Get job result (best-effort)
     * </pre>
     */
    public void getJobResult(com.croupier.sdk.v1.GetJobResultRequest request,
        io.grpc.stub.StreamObserver<com.croupier.sdk.v1.GetJobResultResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetJobResultMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service ClientService.
   * <pre>
   * SDK Client Service - Interface for registering game functions with Agent
   * This is what SDK clients use to register themselves and their functions
   * </pre>
   */
  public static final class ClientServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<ClientServiceBlockingV2Stub> {
    private ClientServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ClientServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ClientServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     * <pre>
     * Register this client and its functions with the Agent
     * </pre>
     */
    public com.croupier.sdk.v1.RegisterClientResponse registerClient(com.croupier.sdk.v1.RegisterClientRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getRegisterClientMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * Send periodic heartbeats to maintain session
     * </pre>
     */
    public com.croupier.sdk.v1.HeartbeatResponse heartbeat(com.croupier.sdk.v1.HeartbeatRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getHeartbeatMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * List all registered clients and their functions
     * </pre>
     */
    public com.croupier.sdk.v1.ListClientsResponse listClients(com.croupier.sdk.v1.ListClientsRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getListClientsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * Get job result (best-effort)
     * </pre>
     */
    public com.croupier.sdk.v1.GetJobResultResponse getJobResult(com.croupier.sdk.v1.GetJobResultRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetJobResultMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service ClientService.
   * <pre>
   * SDK Client Service - Interface for registering game functions with Agent
   * This is what SDK clients use to register themselves and their functions
   * </pre>
   */
  public static final class ClientServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<ClientServiceBlockingStub> {
    private ClientServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ClientServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ClientServiceBlockingStub(channel, callOptions);
    }

    /**
     * <pre>
     * Register this client and its functions with the Agent
     * </pre>
     */
    public com.croupier.sdk.v1.RegisterClientResponse registerClient(com.croupier.sdk.v1.RegisterClientRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getRegisterClientMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * Send periodic heartbeats to maintain session
     * </pre>
     */
    public com.croupier.sdk.v1.HeartbeatResponse heartbeat(com.croupier.sdk.v1.HeartbeatRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getHeartbeatMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * List all registered clients and their functions
     * </pre>
     */
    public com.croupier.sdk.v1.ListClientsResponse listClients(com.croupier.sdk.v1.ListClientsRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getListClientsMethod(), getCallOptions(), request);
    }

    /**
     * <pre>
     * Get job result (best-effort)
     * </pre>
     */
    public com.croupier.sdk.v1.GetJobResultResponse getJobResult(com.croupier.sdk.v1.GetJobResultRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetJobResultMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service ClientService.
   * <pre>
   * SDK Client Service - Interface for registering game functions with Agent
   * This is what SDK clients use to register themselves and their functions
   * </pre>
   */
  public static final class ClientServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<ClientServiceFutureStub> {
    private ClientServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected ClientServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new ClientServiceFutureStub(channel, callOptions);
    }

    /**
     * <pre>
     * Register this client and its functions with the Agent
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.croupier.sdk.v1.RegisterClientResponse> registerClient(
        com.croupier.sdk.v1.RegisterClientRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getRegisterClientMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * Send periodic heartbeats to maintain session
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.croupier.sdk.v1.HeartbeatResponse> heartbeat(
        com.croupier.sdk.v1.HeartbeatRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getHeartbeatMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * List all registered clients and their functions
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.croupier.sdk.v1.ListClientsResponse> listClients(
        com.croupier.sdk.v1.ListClientsRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getListClientsMethod(), getCallOptions()), request);
    }

    /**
     * <pre>
     * Get job result (best-effort)
     * </pre>
     */
    public com.google.common.util.concurrent.ListenableFuture<com.croupier.sdk.v1.GetJobResultResponse> getJobResult(
        com.croupier.sdk.v1.GetJobResultRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getGetJobResultMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_REGISTER_CLIENT = 0;
  private static final int METHODID_HEARTBEAT = 1;
  private static final int METHODID_LIST_CLIENTS = 2;
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
        case METHODID_REGISTER_CLIENT:
          serviceImpl.registerClient((com.croupier.sdk.v1.RegisterClientRequest) request,
              (io.grpc.stub.StreamObserver<com.croupier.sdk.v1.RegisterClientResponse>) responseObserver);
          break;
        case METHODID_HEARTBEAT:
          serviceImpl.heartbeat((com.croupier.sdk.v1.HeartbeatRequest) request,
              (io.grpc.stub.StreamObserver<com.croupier.sdk.v1.HeartbeatResponse>) responseObserver);
          break;
        case METHODID_LIST_CLIENTS:
          serviceImpl.listClients((com.croupier.sdk.v1.ListClientsRequest) request,
              (io.grpc.stub.StreamObserver<com.croupier.sdk.v1.ListClientsResponse>) responseObserver);
          break;
        case METHODID_GET_JOB_RESULT:
          serviceImpl.getJobResult((com.croupier.sdk.v1.GetJobResultRequest) request,
              (io.grpc.stub.StreamObserver<com.croupier.sdk.v1.GetJobResultResponse>) responseObserver);
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
          getRegisterClientMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.croupier.sdk.v1.RegisterClientRequest,
              com.croupier.sdk.v1.RegisterClientResponse>(
                service, METHODID_REGISTER_CLIENT)))
        .addMethod(
          getHeartbeatMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.croupier.sdk.v1.HeartbeatRequest,
              com.croupier.sdk.v1.HeartbeatResponse>(
                service, METHODID_HEARTBEAT)))
        .addMethod(
          getListClientsMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.croupier.sdk.v1.ListClientsRequest,
              com.croupier.sdk.v1.ListClientsResponse>(
                service, METHODID_LIST_CLIENTS)))
        .addMethod(
          getGetJobResultMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.croupier.sdk.v1.GetJobResultRequest,
              com.croupier.sdk.v1.GetJobResultResponse>(
                service, METHODID_GET_JOB_RESULT)))
        .build();
  }

  private static abstract class ClientServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    ClientServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return com.croupier.sdk.v1.ClientProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("ClientService");
    }
  }

  private static final class ClientServiceFileDescriptorSupplier
      extends ClientServiceBaseDescriptorSupplier {
    ClientServiceFileDescriptorSupplier() {}
  }

  private static final class ClientServiceMethodDescriptorSupplier
      extends ClientServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    ClientServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (ClientServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new ClientServiceFileDescriptorSupplier())
              .addMethod(getRegisterClientMethod())
              .addMethod(getHeartbeatMethod())
              .addMethod(getListClientsMethod())
              .addMethod(getGetJobResultMethod())
              .build();
        }
      }
    }
    return result;
  }
}
