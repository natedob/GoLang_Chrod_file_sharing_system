
# Distributed systems Lab3

## Chord file sharing system

### Build the program

go build -o chord main.go             --Bygg programmet och ge det namnet chord

sudo mv chord /usr/local/bin/           -- Flytta det körbara chord till bin vilket ligger i PATH.

### Start processes

chord -a 127.0.0.1 -p 1111 --ts 3000 --tff 1000 --tcp 3000 -r 4 -m 7          (ID 115 om m = 7)                      CREATE

chord -a 127.0.0.1 -p 3333 --ja 127.0.0.1 --jp 1111 --ts 3000 --tff 1000 --tcp 3000 -r 4    (ID 5   om m = 7)          JOIN

chord -a 127.0.0.1 -p 2222 --ja 127.0.0.1 --jp 1111 --ts 3000 --tff 1000 --tcp 3000 -r 4    (ID 15   om m = 7)        JOIN

chord -a 127.0.0.1 -p 4444 --ja 127.0.0.1 --jp 1111 --ts 3000 --tff 1000 --tcp 3000 -r 4    (ID 52   om m = 7)         JOIN

chord -a 127.0.0.1 -p 4400 --ja 127.0.0.1 --jp 1111 --ts 3000 --tff 1000 --tcp 3000 -r 4    (ID 73   om m = 7)         JOIN

### Commands

PrintState

Lookup

StoreFile

### Expected results

c.txt       (ID 11   om m = 7)
ciao.txt    (ID 18   om m = 7)
hej.txt     (ID 33   om m = 7)
hejsan.txt  (ID 48   om m = 7)
9.12.txt    (ID 48   om m = 7)
hallo.txt   (ID 63   om m = 7)
ö.txt       (ID 73   om m = 7)
haloj.txt   (ID 120   om m = 7)


m | 2^m
0 | 1
1 | 2
2 | 4
3 | 8
4 | 16
5 | 32
6 | 64
7 | 128
8 | 256
9 | 512
10 | 1024
11 | 2048
12 | 4096
13 | 8192
14 | 16384
15 | 32768
16 | 65536
17 | 131072
18 | 262144
19 | 524288
20 | 1048576

