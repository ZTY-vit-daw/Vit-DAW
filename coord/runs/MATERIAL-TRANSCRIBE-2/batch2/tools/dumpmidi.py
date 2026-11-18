# -*- coding: utf-8 -*-
import sys, struct
data=open(sys.argv[1],'rb').read()
print('len',len(data),'head',data[:14].hex())
pos=14
while pos+8<=len(data):
    tag=data[pos:pos+4]; ln=struct.unpack('>I',data[pos+4:pos+8])[0]
    print('chunk',tag,'len',ln)
    body=data[pos+8:pos+8+ln]
    print('body hex:',body.hex())
    pos+=8+ln
