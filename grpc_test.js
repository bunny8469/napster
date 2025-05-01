import grpc from "k6/net/grpc";
import { check, sleep } from "k6";

const client = new grpc.Client();
client.load(["."], "napster.proto");  // Adjust path to your proto file

export const options = {
  stages: [
    { duration: '10s', target: 10 }, // ramp-up
    { duration: '30s', target: 10 }, // steady
    { duration: '10s', target: 0 },  // ramp-down
  ],
};

export default () => {
  client.connect("localhost:50051", {
    plaintext: true,
  });

  const response = client.invoke("napster.CentralServer/SearchFile", {
    query: "Mantra"
  });
  
  check(response, {
    "status is OK": (r) => r && r.status === grpc.StatusOK,
    "returned array": (r) => r && r.message.results && r.message.results.length > 0,
  });

  client.close();
  sleep(1);
};
