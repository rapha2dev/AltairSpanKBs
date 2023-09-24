import datetime, time
import sys

def hanoi(n):
    if n == 1:
        return 1
    else:
        return hanoi(n-1) + hanoi(n-1) + 1    

t = datetime.datetime.now()
res = hanoi(25)
print("OK: " + str(res) if res == 33554431 else "WRONG: " + str(res))
print("time:", (datetime.datetime.now()-t))