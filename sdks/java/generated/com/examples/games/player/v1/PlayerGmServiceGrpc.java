package com.examples.games.player.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class PlayerGmServiceGrpc {

  private PlayerGmServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "examples.games.player.v1.PlayerGmService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<com.examples.games.player.v1.BanRequest,
      com.examples.games.player.v1.BanResponse> getBanMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "Ban",
      requestType = com.examples.games.player.v1.BanRequest.class,
      responseType = com.examples.games.player.v1.BanResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.examples.games.player.v1.BanRequest,
      com.examples.games.player.v1.BanResponse> getBanMethod() {
    io.grpc.MethodDescriptor<com.examples.games.player.v1.BanRequest, com.examples.games.player.v1.BanResponse> getBanMethod;
    if ((getBanMethod = PlayerGmServiceGrpc.getBanMethod) == null) {
      synchronized (PlayerGmServiceGrpc.class) {
        if ((getBanMethod = PlayerGmServiceGrpc.getBanMethod) == null) {
          PlayerGmServiceGrpc.getBanMethod = getBanMethod =
              io.grpc.MethodDescriptor.<com.examples.games.player.v1.BanRequest, com.examples.games.player.v1.BanResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "Ban"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.examples.games.player.v1.BanRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.examples.games.player.v1.BanResponse.getDefaultInstance()))
              .setSchemaDescriptor(new PlayerGmServiceMethodDescriptorSupplier("Ban"))
              .build();
        }
      }
    }
    return getBanMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static PlayerGmServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PlayerGmServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PlayerGmServiceStub>() {
        @java.lang.Override
        public PlayerGmServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PlayerGmServiceStub(channel, callOptions);
        }
      };
    return PlayerGmServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static PlayerGmServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PlayerGmServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PlayerGmServiceBlockingV2Stub>() {
        @java.lang.Override
        public PlayerGmServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PlayerGmServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return PlayerGmServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static PlayerGmServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PlayerGmServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PlayerGmServiceBlockingStub>() {
        @java.lang.Override
        public PlayerGmServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PlayerGmServiceBlockingStub(channel, callOptions);
        }
      };
    return PlayerGmServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static PlayerGmServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PlayerGmServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PlayerGmServiceFutureStub>() {
        @java.lang.Override
        public PlayerGmServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PlayerGmServiceFutureStub(channel, callOptions);
        }
      };
    return PlayerGmServiceFutureStub.newStub(factory, channel);
  }

  /**
   */
  public interface AsyncService {

    /**
     */
    default void ban(com.examples.games.player.v1.BanRequest request,
        io.grpc.stub.StreamObserver<com.examples.games.player.v1.BanResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getBanMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service PlayerGmService.
   */
  public static abstract class PlayerGmServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return PlayerGmServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service PlayerGmService.
   */
  public static final class PlayerGmServiceStub
      extends io.grpc.stub.AbstractAsyncStub<PlayerGmServiceStub> {
    private PlayerGmServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PlayerGmServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PlayerGmServiceStub(channel, callOptions);
    }

    /**
     */
    public void ban(com.examples.games.player.v1.BanRequest request,
        io.grpc.stub.StreamObserver<com.examples.games.player.v1.BanResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getBanMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service PlayerGmService.
   */
  public static final class PlayerGmServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<PlayerGmServiceBlockingV2Stub> {
    private PlayerGmServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PlayerGmServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PlayerGmServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     */
    public com.examples.games.player.v1.BanResponse ban(com.examples.games.player.v1.BanRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getBanMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service PlayerGmService.
   */
  public static final class PlayerGmServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<PlayerGmServiceBlockingStub> {
    private PlayerGmServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PlayerGmServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PlayerGmServiceBlockingStub(channel, callOptions);
    }

    /**
     */
    public com.examples.games.player.v1.BanResponse ban(com.examples.games.player.v1.BanRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getBanMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service PlayerGmService.
   */
  public static final class PlayerGmServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<PlayerGmServiceFutureStub> {
    private PlayerGmServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PlayerGmServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PlayerGmServiceFutureStub(channel, callOptions);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.examples.games.player.v1.BanResponse> ban(
        com.examples.games.player.v1.BanRequest request) {
      return io.grpc.stub.ClientCalls.futureUnaryCall(
          getChannel().newCall(getBanMethod(), getCallOptions()), request);
    }
  }

  private static final int METHODID_BAN = 0;

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
        case METHODID_BAN:
          serviceImpl.ban((com.examples.games.player.v1.BanRequest) request,
              (io.grpc.stub.StreamObserver<com.examples.games.player.v1.BanResponse>) responseObserver);
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
          getBanMethod(),
          io.grpc.stub.ServerCalls.asyncUnaryCall(
            new MethodHandlers<
              com.examples.games.player.v1.BanRequest,
              com.examples.games.player.v1.BanResponse>(
                service, METHODID_BAN)))
        .build();
  }

  private static abstract class PlayerGmServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    PlayerGmServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return com.examples.games.player.v1.PlayerProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("PlayerGmService");
    }
  }

  private static final class PlayerGmServiceFileDescriptorSupplier
      extends PlayerGmServiceBaseDescriptorSupplier {
    PlayerGmServiceFileDescriptorSupplier() {}
  }

  private static final class PlayerGmServiceMethodDescriptorSupplier
      extends PlayerGmServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    PlayerGmServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (PlayerGmServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new PlayerGmServiceFileDescriptorSupplier())
              .addMethod(getBanMethod())
              .build();
        }
      }
    }
    return result;
  }
}
