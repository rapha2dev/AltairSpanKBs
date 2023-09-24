import datetime, time
import sys
sys.setrecursionlimit(10**6)

def mult(a, b, c):
    if c > 1:                      
        return mult(a, b + a, c - 1)
    else:
        return b

def factorial(n):    
    if n == 1:
        return n
    else:               
        return mult(n, n, factorial(n - 1))       


t = datetime.datetime.now()
res = factorial(10)
print("OK: " + str(res) if res == 3628800 else "WRONG: " + str(res))
print("time:", (datetime.datetime.now()-t))