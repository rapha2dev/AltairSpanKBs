import datetime, time
import sys
sys.setrecursionlimit(10**6)

def sum(n):
    if n == 1:
        return n
    else:
        return n + sum(n - 1)

t = datetime.datetime.now()
res = sum(50000)
print("OK: " + str(res) if res == 1250025000 else "WRONG: " + str(res))
print("time:", (datetime.datetime.now()-t))