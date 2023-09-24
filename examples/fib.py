import datetime, time
import sys
sys.setrecursionlimit(10**6)
def fib(n):
    if n < 2:
        return n
    else:
        return fib(n - 1) + fib(n - 2)  

t = datetime.datetime.now()
res = fib(35)
print("OK: " + str(res) if res == 9227465 else "WRONG: " + str(res))
print("time:", (datetime.datetime.now()-t))