using Go = import "/go.capnp";
@0x85d3acc39d94e0f8;
$Go.package("request");
$Go.import("request");

struct SessionOpenRequest {
    destAddr @0: Text;
}

struct SessionOpenResponse {
    enum Status {
        ok @0;
        dialFail @1;
        error @2;
    }
    
    status @0: Status;
    id @1: UInt64;
}

struct PollRequest {
    id @0: UInt64;
}

struct PollResponse {
    enum Status {
        pollOK @0;
        noData @1;
        closed @2;
        error @3;
    }

    status @0: Status;
    id @1: UInt64;
}

struct WriteRequest {
    id @0: UInt64;
    fragHeader @1: UInt16;
    data @2: Data;
}

struct WriteResponse {
    enum Status {
        writeOk @0;
        closed @1;
        error @2;
    }

    status @0: Status;
}