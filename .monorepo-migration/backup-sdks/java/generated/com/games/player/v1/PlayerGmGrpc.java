package com.games.player.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class PlayerGmGrpc {

  private PlayerGmGrpc() {}

  public static final java.lang.String SERVICE_NAME = "games.player.v1.PlayerGm";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<com.games.player.v1.BanRequest,
      com.games.player.v1.BanResponse> getBanMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "Ban",
      requestType = com.games.player.v1.BanRequest.class,
      responseType = com.games.player.v1.BanResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.games.player.v1.BanRequest,
      com.games.player.v1.BanResponse> getBanMethod() {
    io.grpc.MethodDescriptor<com.games.player.v1.BanRequest, com.games.player.v1.BanResponse> getBanMethod;
    if ((getBanMethod = PlayerGmGrpc.getBanMethod) == null) {
      synchronized (PlayerGmGrpc.class) {
        if ((getBanMethod = PlayerGmGrpc.getBanMethod) == null) {
          PlayerGmGrpc.getBanMethod = getBanMethod =
              io.grpc.MethodDescriptor.<com.games.player.v1.BanRequest, com.games.player.v1.BanResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "Ban"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.games.player.v1.BanRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.games.player.v1.BanResponse.getDefaultInstance()))
              .setSchemaDescriptor(new PlayerGmMethodDescriptorSupplier("Ban"))
              .build();
        }
      }
    }
    return getBanMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static PlayerGmStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PlayerGmStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PlayerGmStub>() {
        @java.lang.Override
        public PlayerGmStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PlayerGmStub(channel, callOptions);
        }
      };
    return PlayerGmStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static PlayerGmBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PlayerGmBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PlayerGmBlockingV2Stub>() {
        @java.lang.Override
        public PlayerGmBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PlayerGmBlockingV2Stub(channel, callOptions);
        }
      };
    return PlayerGmBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static PlayerGmBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PlayerGmBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PlayerGmBlockingStub>() {
        @java.lang.Override
        public PlayerGmBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PlayerGmBlockingStub(channel, callOptions);
        }
      };
    return PlayerGmBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static PlayerGmFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<PlayerGmFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<PlayerGmFutureStub>() {
        @java.lang.Override
        public PlayerGmFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new PlayerGmFutureStub(channel, callOptions);
        }
      };
    return PlayerGmFutureStub.newStub(factory, channel);
  }

  /**
   */
  public interface AsyncService {

    /**
     */
    default void ban(com.games.player.v1.BanRequest request,
        io.grpc.stub.StreamObserver<com.games.player.v1.BanResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getBanMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service PlayerGm.
   */
  public static abstract class PlayerGmImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return PlayerGmGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service PlayerGm.
   */
  public static final class PlayerGmStub
      extends io.grpc.stub.AbstractAsyncStub<PlayerGmStub> {
    private PlayerGmStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PlayerGmStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PlayerGmStub(channel, callOptions);
    }

    /**
     */
    public void ban(com.games.player.v1.BanRequest request,
        io.grpc.stub.StreamObserver<com.games.player.v1.BanResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getBanMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service PlayerGm.
   */
  public static final class PlayerGmBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<PlayerGmBlockingV2Stub> {
    private PlayerGmBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PlayerGmBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PlayerGmBlockingV2Stub(channel, callOptions);
    }

    /**
     */
    public com.games.player.v1.BanResponse ban(com.games.player.v1.BanRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getBanMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service PlayerGm.
   */
  public static final class PlayerGmBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<PlayerGmBlockingStub> {
    private PlayerGmBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PlayerGmBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PlayerGmBlockingStub(channel, callOptions);
    }

    /**
     */
    public com.games.player.v1.BanResponse ban(com.games.player.v1.BanRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getBanMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service PlayerGm.
   */
  public static final class PlayerGmFutureStub
      extends io.grpc.stub.AbstractFutureStub<PlayerGmFutureStub> {
    private PlayerGmFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected PlayerGmFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new PlayerGmFutureStub(channel, callOptions);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.games.player.v1.BanResponse> ban(
        com.games.player.v1.BanRequest request) {
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
          serviceImpl.ban((com.games.player.v1.BanRequest) request,
              (io.grpc.stub.StreamObserver<com.games.player.v1.BanResponse>) responseObserver);
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
              com.games.player.v1.BanRequest,
              com.games.player.v1.BanResponse>(
                service, METHODID_BAN)))
        .build();
  }

  private static abstract class PlayerGmBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    PlayerGmBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return com.games.player.v1.PlayerProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("PlayerGm");
    }
  }

  private static final class PlayerGmFileDescriptorSupplier
      extends PlayerGmBaseDescriptorSupplier {
    PlayerGmFileDescriptorSupplier() {}
  }

  private static final class PlayerGmMethodDescriptorSupplier
      extends PlayerGmBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    PlayerGmMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (PlayerGmGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new PlayerGmFileDescriptorSupplier())
              .addMethod(getBanMethod())
              .build();
        }
      }
    }
    return result;
  }
}
