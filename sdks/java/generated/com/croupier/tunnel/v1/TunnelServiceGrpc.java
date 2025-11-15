package com.croupier.tunnel.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class TunnelServiceGrpc {

  private TunnelServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "croupier.tunnel.v1.TunnelService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<com.croupier.tunnel.v1.TunnelMessage,
      com.croupier.tunnel.v1.TunnelMessage> getOpenMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "Open",
      requestType = com.croupier.tunnel.v1.TunnelMessage.class,
      responseType = com.croupier.tunnel.v1.TunnelMessage.class,
      methodType = io.grpc.MethodDescriptor.MethodType.BIDI_STREAMING)
  public static io.grpc.MethodDescriptor<com.croupier.tunnel.v1.TunnelMessage,
      com.croupier.tunnel.v1.TunnelMessage> getOpenMethod() {
    io.grpc.MethodDescriptor<com.croupier.tunnel.v1.TunnelMessage, com.croupier.tunnel.v1.TunnelMessage> getOpenMethod;
    if ((getOpenMethod = TunnelServiceGrpc.getOpenMethod) == null) {
      synchronized (TunnelServiceGrpc.class) {
        if ((getOpenMethod = TunnelServiceGrpc.getOpenMethod) == null) {
          TunnelServiceGrpc.getOpenMethod = getOpenMethod =
              io.grpc.MethodDescriptor.<com.croupier.tunnel.v1.TunnelMessage, com.croupier.tunnel.v1.TunnelMessage>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.BIDI_STREAMING)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "Open"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.tunnel.v1.TunnelMessage.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.tunnel.v1.TunnelMessage.getDefaultInstance()))
              .setSchemaDescriptor(new TunnelServiceMethodDescriptorSupplier("Open"))
              .build();
        }
      }
    }
    return getOpenMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static TunnelServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<TunnelServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<TunnelServiceStub>() {
        @java.lang.Override
        public TunnelServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new TunnelServiceStub(channel, callOptions);
        }
      };
    return TunnelServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static TunnelServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<TunnelServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<TunnelServiceBlockingV2Stub>() {
        @java.lang.Override
        public TunnelServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new TunnelServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return TunnelServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static TunnelServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<TunnelServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<TunnelServiceBlockingStub>() {
        @java.lang.Override
        public TunnelServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new TunnelServiceBlockingStub(channel, callOptions);
        }
      };
    return TunnelServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static TunnelServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<TunnelServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<TunnelServiceFutureStub>() {
        @java.lang.Override
        public TunnelServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new TunnelServiceFutureStub(channel, callOptions);
        }
      };
    return TunnelServiceFutureStub.newStub(factory, channel);
  }

  /**
   */
  public interface AsyncService {

    /**
     */
    default io.grpc.stub.StreamObserver<com.croupier.tunnel.v1.TunnelMessage> open(
        io.grpc.stub.StreamObserver<com.croupier.tunnel.v1.TunnelMessage> responseObserver) {
      return io.grpc.stub.ServerCalls.asyncUnimplementedStreamingCall(getOpenMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service TunnelService.
   */
  public static abstract class TunnelServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return TunnelServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service TunnelService.
   */
  public static final class TunnelServiceStub
      extends io.grpc.stub.AbstractAsyncStub<TunnelServiceStub> {
    private TunnelServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected TunnelServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new TunnelServiceStub(channel, callOptions);
    }

    /**
     */
    public io.grpc.stub.StreamObserver<com.croupier.tunnel.v1.TunnelMessage> open(
        io.grpc.stub.StreamObserver<com.croupier.tunnel.v1.TunnelMessage> responseObserver) {
      return io.grpc.stub.ClientCalls.asyncBidiStreamingCall(
          getChannel().newCall(getOpenMethod(), getCallOptions()), responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service TunnelService.
   */
  public static final class TunnelServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<TunnelServiceBlockingV2Stub> {
    private TunnelServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected TunnelServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new TunnelServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     */
    @io.grpc.ExperimentalApi("https://github.com/grpc/grpc-java/issues/10918")
    public io.grpc.stub.BlockingClientCall<com.croupier.tunnel.v1.TunnelMessage, com.croupier.tunnel.v1.TunnelMessage>
        open() {
      return io.grpc.stub.ClientCalls.blockingBidiStreamingCall(
          getChannel(), getOpenMethod(), getCallOptions());
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service TunnelService.
   */
  public static final class TunnelServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<TunnelServiceBlockingStub> {
    private TunnelServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected TunnelServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new TunnelServiceBlockingStub(channel, callOptions);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service TunnelService.
   */
  public static final class TunnelServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<TunnelServiceFutureStub> {
    private TunnelServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected TunnelServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new TunnelServiceFutureStub(channel, callOptions);
    }
  }

  private static final int METHODID_OPEN = 0;

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
        default:
          throw new AssertionError();
      }
    }

    @java.lang.Override
    @java.lang.SuppressWarnings("unchecked")
    public io.grpc.stub.StreamObserver<Req> invoke(
        io.grpc.stub.StreamObserver<Resp> responseObserver) {
      switch (methodId) {
        case METHODID_OPEN:
          return (io.grpc.stub.StreamObserver<Req>) serviceImpl.open(
              (io.grpc.stub.StreamObserver<com.croupier.tunnel.v1.TunnelMessage>) responseObserver);
        default:
          throw new AssertionError();
      }
    }
  }

  public static final io.grpc.ServerServiceDefinition bindService(AsyncService service) {
    return io.grpc.ServerServiceDefinition.builder(getServiceDescriptor())
        .addMethod(
          getOpenMethod(),
          io.grpc.stub.ServerCalls.asyncBidiStreamingCall(
            new MethodHandlers<
              com.croupier.tunnel.v1.TunnelMessage,
              com.croupier.tunnel.v1.TunnelMessage>(
                service, METHODID_OPEN)))
        .build();
  }

  private static abstract class TunnelServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    TunnelServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return com.croupier.tunnel.v1.TunnelProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("TunnelService");
    }
  }

  private static final class TunnelServiceFileDescriptorSupplier
      extends TunnelServiceBaseDescriptorSupplier {
    TunnelServiceFileDescriptorSupplier() {}
  }

  private static final class TunnelServiceMethodDescriptorSupplier
      extends TunnelServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    TunnelServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (TunnelServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new TunnelServiceFileDescriptorSupplier())
              .addMethod(getOpenMethod())
              .build();
        }
      }
    }
    return result;
  }
}
