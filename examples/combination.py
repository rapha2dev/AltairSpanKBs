import datetime, time
import sys

def combination(n, k):
    a = k == 0
    b = k == n
    if a or b:
        return 1
    else:
        return combination(n - 1, k - 1) + combination(n - 1, k)

t = datetime.datetime.now()
res = combination(50, 5)
print("OK: " + str(res) if res == 2118760 else "WRONG: " + str(res))
print("time:", (datetime.datetime.now()-t))