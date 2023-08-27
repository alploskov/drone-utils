import io
import time, os, sys
import struct
import random
from pymavlink import mavutil
from io import BytesIO as SIO


# opcodes
OP_None = 0
OP_TerminateSession = 1
OP_ResetSessions = 2
OP_ListDirectory = 3
OP_OpenFileRO = 4
OP_ReadFile = 5
OP_CreateFile = 6
OP_WriteFile = 7
OP_RemoveFile = 8
OP_CreateDirectory = 9
OP_RemoveDirectory = 10
OP_OpenFileWO = 11
OP_TruncateFile = 12
OP_Rename = 13
OP_CalcFileCRC32 = 14
OP_BurstReadFile = 15
OP_Ack = 128
OP_Nack = 129

# error codes
ERR_None = 0
ERR_Fail = 1
ERR_FailErrno = 2
ERR_InvalidDataSize = 3
ERR_InvalidSession = 4
ERR_NoSessionsAvailable = 5
ERR_EndOfFile = 6
ERR_UnknownCommand = 7
ERR_FileExists = 8
ERR_FileProtected = 9
ERR_FileNotFound = 10

HDR_Len = 12
MAX_Payload = 239

def is_ack(transfer) -> bool:
    return transfer.payload[3] == OP_Ack

class FTP_OP:
    def __init__(self, seq, session, opcode, size, req_opcode, burst_complete, offset, payload):
        self.seq = seq
        self.session = session
        self.opcode = opcode
        self.size = size
        self.req_opcode = req_opcode
        self.burst_complete = burst_complete
        self.offset = offset
        self.payload = payload

    def pack(self):
        '''pack message'''
        ret = struct.pack("<HBBBBBBI", self.seq, self.session, self.opcode, self.size, self.req_opcode, self.burst_complete, 0, self.offset)
        if self.payload is not None:
            ret += self.payload
        ret = bytearray(ret)
        return ret

    def __str__(self):
        plen = 0
        if self.payload is not None:
            plen = len(self.payload)
        ret = "OP seq:%u sess:%u opcode:%d req_opcode:%u size:%u bc:%u ofs:%u plen=%u" % (
            self.seq,
            self.session,
            self.opcode,
            self.req_opcode,
            self.size,
            self.burst_complete,
            self.offset,
            plen)
        if plen > 0:
            ret += " [%u]" % self.payload[0]
        return ret

class FTP:
    def __init__(self, conn):
        self.conn = conn
        self.seq = 0
        self.session = 0
        self.network = 0
        self.last_op = None
        self.last_op_time = time.time()

    def send(self, op):
        '''send a request'''
        op.seq = self.seq
        payload = op.pack()
        plen = len(payload)
        if plen < MAX_Payload + HDR_Len:
            payload.extend(bytearray([0]*((HDR_Len+MAX_Payload)-plen)))
        self.conn.mav.file_transfer_protocol_send(self.network, self.conn.target_system, self.conn.target_component, payload)
        self.seq = (self.seq + 1) % 256
        self.last_op = op
        now = time.time()
        self.last_op_time = time.time()

    def write_file(self, _file, filename):
        BLOCK_SIZE = 100
        filename = str(filename)
        enc_fname = bytearray(filename, 'ascii')
        self.send(FTP_OP(self.seq, self.session, OP_CreateFile, len(enc_fname), 0, 0, 0, enc_fname))
        answer = self.conn.recv_match(type="FILE_TRANSFER_PROTOCOL", blocking=True)
        if not is_ack(answer):
            print('file is not created')
        else:
            print(f'create file {filename}.')
        with open(_file, 'rb') as f:
            data = f.read()
            offst = 0
            while offst + BLOCK_SIZE < len(data):
                self.send(FTP_OP(self.seq, self.session, OP_WriteFile, BLOCK_SIZE, 0, 0, offst, data[offst:offst + 100]))
                offst += BLOCK_SIZE
            else:
                self.send(FTP_OP(self.seq, self.session, OP_WriteFile, len(data[offst:]), 0, 0, offst, data[offst:]))
        self.send(FTP_OP(self.seq, self.session, OP_TerminateSession, 0, 0, 0, 0, None))
