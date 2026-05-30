type MockServer = {
  close: () => void;
};

type StartMockOptions = {
  port?: number;
  scene?: string;
};

export async function startMock(_opts: StartMockOptions): Promise<MockServer> {
  return {
    close: () => {},
  };
}
