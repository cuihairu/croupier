package com.croupier.edge.job.v1;

import static io.grpc.MethodDescriptor.generateFullMethodName;

/**
 */
@io.grpc.stub.annotations.GrpcGenerated
public final class JobServiceGrpc {

  private JobServiceGrpc() {}

  public static final java.lang.String SERVICE_NAME = "croupier.edge.job.v1.JobService";

  // Static method descriptors that strictly reflect the proto.
  private static volatile io.grpc.MethodDescriptor<com.croupier.edge.job.v1.GetJobResultRequest,
      com.croupier.edge.job.v1.GetJobResultResponse> getGetJobResultMethod;

  @io.grpc.stub.annotations.RpcMethod(
      fullMethodName = SERVICE_NAME + '/' + "GetJobResult",
      requestType = com.croupier.edge.job.v1.GetJobResultRequest.class,
      responseType = com.croupier.edge.job.v1.GetJobResultResponse.class,
      methodType = io.grpc.MethodDescriptor.MethodType.UNARY)
  public static io.grpc.MethodDescriptor<com.croupier.edge.job.v1.GetJobResultRequest,
      com.croupier.edge.job.v1.GetJobResultResponse> getGetJobResultMethod() {
    io.grpc.MethodDescriptor<com.croupier.edge.job.v1.GetJobResultRequest, com.croupier.edge.job.v1.GetJobResultResponse> getGetJobResultMethod;
    if ((getGetJobResultMethod = JobServiceGrpc.getGetJobResultMethod) == null) {
      synchronized (JobServiceGrpc.class) {
        if ((getGetJobResultMethod = JobServiceGrpc.getGetJobResultMethod) == null) {
          JobServiceGrpc.getGetJobResultMethod = getGetJobResultMethod =
              io.grpc.MethodDescriptor.<com.croupier.edge.job.v1.GetJobResultRequest, com.croupier.edge.job.v1.GetJobResultResponse>newBuilder()
              .setType(io.grpc.MethodDescriptor.MethodType.UNARY)
              .setFullMethodName(generateFullMethodName(SERVICE_NAME, "GetJobResult"))
              .setSampledToLocalTracing(true)
              .setRequestMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.edge.job.v1.GetJobResultRequest.getDefaultInstance()))
              .setResponseMarshaller(io.grpc.protobuf.ProtoUtils.marshaller(
                  com.croupier.edge.job.v1.GetJobResultResponse.getDefaultInstance()))
              .setSchemaDescriptor(new JobServiceMethodDescriptorSupplier("GetJobResult"))
              .build();
        }
      }
    }
    return getGetJobResultMethod;
  }

  /**
   * Creates a new async stub that supports all call types for the service
   */
  public static JobServiceStub newStub(io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<JobServiceStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<JobServiceStub>() {
        @java.lang.Override
        public JobServiceStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new JobServiceStub(channel, callOptions);
        }
      };
    return JobServiceStub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports all types of calls on the service
   */
  public static JobServiceBlockingV2Stub newBlockingV2Stub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<JobServiceBlockingV2Stub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<JobServiceBlockingV2Stub>() {
        @java.lang.Override
        public JobServiceBlockingV2Stub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new JobServiceBlockingV2Stub(channel, callOptions);
        }
      };
    return JobServiceBlockingV2Stub.newStub(factory, channel);
  }

  /**
   * Creates a new blocking-style stub that supports unary and streaming output calls on the service
   */
  public static JobServiceBlockingStub newBlockingStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<JobServiceBlockingStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<JobServiceBlockingStub>() {
        @java.lang.Override
        public JobServiceBlockingStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new JobServiceBlockingStub(channel, callOptions);
        }
      };
    return JobServiceBlockingStub.newStub(factory, channel);
  }

  /**
   * Creates a new ListenableFuture-style stub that supports unary calls on the service
   */
  public static JobServiceFutureStub newFutureStub(
      io.grpc.Channel channel) {
    io.grpc.stub.AbstractStub.StubFactory<JobServiceFutureStub> factory =
      new io.grpc.stub.AbstractStub.StubFactory<JobServiceFutureStub>() {
        @java.lang.Override
        public JobServiceFutureStub newStub(io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
          return new JobServiceFutureStub(channel, callOptions);
        }
      };
    return JobServiceFutureStub.newStub(factory, channel);
  }

  /**
   */
  public interface AsyncService {

    /**
     */
    default void getJobResult(com.croupier.edge.job.v1.GetJobResultRequest request,
        io.grpc.stub.StreamObserver<com.croupier.edge.job.v1.GetJobResultResponse> responseObserver) {
      io.grpc.stub.ServerCalls.asyncUnimplementedUnaryCall(getGetJobResultMethod(), responseObserver);
    }
  }

  /**
   * Base class for the server implementation of the service JobService.
   */
  public static abstract class JobServiceImplBase
      implements io.grpc.BindableService, AsyncService {

    @java.lang.Override public final io.grpc.ServerServiceDefinition bindService() {
      return JobServiceGrpc.bindService(this);
    }
  }

  /**
   * A stub to allow clients to do asynchronous rpc calls to service JobService.
   */
  public static final class JobServiceStub
      extends io.grpc.stub.AbstractAsyncStub<JobServiceStub> {
    private JobServiceStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected JobServiceStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new JobServiceStub(channel, callOptions);
    }

    /**
     */
    public void getJobResult(com.croupier.edge.job.v1.GetJobResultRequest request,
        io.grpc.stub.StreamObserver<com.croupier.edge.job.v1.GetJobResultResponse> responseObserver) {
      io.grpc.stub.ClientCalls.asyncUnaryCall(
          getChannel().newCall(getGetJobResultMethod(), getCallOptions()), request, responseObserver);
    }
  }

  /**
   * A stub to allow clients to do synchronous rpc calls to service JobService.
   */
  public static final class JobServiceBlockingV2Stub
      extends io.grpc.stub.AbstractBlockingStub<JobServiceBlockingV2Stub> {
    private JobServiceBlockingV2Stub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected JobServiceBlockingV2Stub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new JobServiceBlockingV2Stub(channel, callOptions);
    }

    /**
     */
    public com.croupier.edge.job.v1.GetJobResultResponse getJobResult(com.croupier.edge.job.v1.GetJobResultRequest request) throws io.grpc.StatusException {
      return io.grpc.stub.ClientCalls.blockingV2UnaryCall(
          getChannel(), getGetJobResultMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do limited synchronous rpc calls to service JobService.
   */
  public static final class JobServiceBlockingStub
      extends io.grpc.stub.AbstractBlockingStub<JobServiceBlockingStub> {
    private JobServiceBlockingStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected JobServiceBlockingStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new JobServiceBlockingStub(channel, callOptions);
    }

    /**
     */
    public com.croupier.edge.job.v1.GetJobResultResponse getJobResult(com.croupier.edge.job.v1.GetJobResultRequest request) {
      return io.grpc.stub.ClientCalls.blockingUnaryCall(
          getChannel(), getGetJobResultMethod(), getCallOptions(), request);
    }
  }

  /**
   * A stub to allow clients to do ListenableFuture-style rpc calls to service JobService.
   */
  public static final class JobServiceFutureStub
      extends io.grpc.stub.AbstractFutureStub<JobServiceFutureStub> {
    private JobServiceFutureStub(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      super(channel, callOptions);
    }

    @java.lang.Override
    protected JobServiceFutureStub build(
        io.grpc.Channel channel, io.grpc.CallOptions callOptions) {
      return new JobServiceFutureStub(channel, callOptions);
    }

    /**
     */
    public com.google.common.util.concurrent.ListenableFuture<com.croupier.edge.job.v1.GetJobResultResponse> getJobResult(
        com.croupier.edge.job.v1.GetJobResultRequest request) {
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
          serviceImpl.getJobResult((com.croupier.edge.job.v1.GetJobResultRequest) request,
              (io.grpc.stub.StreamObserver<com.croupier.edge.job.v1.GetJobResultResponse>) responseObserver);
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
              com.croupier.edge.job.v1.GetJobResultRequest,
              com.croupier.edge.job.v1.GetJobResultResponse>(
                service, METHODID_GET_JOB_RESULT)))
        .build();
  }

  private static abstract class JobServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoFileDescriptorSupplier, io.grpc.protobuf.ProtoServiceDescriptorSupplier {
    JobServiceBaseDescriptorSupplier() {}

    @java.lang.Override
    public com.google.protobuf.Descriptors.FileDescriptor getFileDescriptor() {
      return com.croupier.edge.job.v1.JobProto.getDescriptor();
    }

    @java.lang.Override
    public com.google.protobuf.Descriptors.ServiceDescriptor getServiceDescriptor() {
      return getFileDescriptor().findServiceByName("JobService");
    }
  }

  private static final class JobServiceFileDescriptorSupplier
      extends JobServiceBaseDescriptorSupplier {
    JobServiceFileDescriptorSupplier() {}
  }

  private static final class JobServiceMethodDescriptorSupplier
      extends JobServiceBaseDescriptorSupplier
      implements io.grpc.protobuf.ProtoMethodDescriptorSupplier {
    private final java.lang.String methodName;

    JobServiceMethodDescriptorSupplier(java.lang.String methodName) {
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
      synchronized (JobServiceGrpc.class) {
        result = serviceDescriptor;
        if (result == null) {
          serviceDescriptor = result = io.grpc.ServiceDescriptor.newBuilder(SERVICE_NAME)
              .setSchemaDescriptor(new JobServiceFileDescriptorSupplier())
              .addMethod(getGetJobResultMethod())
              .build();
        }
      }
    }
    return result;
  }
}
