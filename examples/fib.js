eval(`function fib(n){
    if (n < 2) {
        return n
    } else {
        return fib(n - 1) + fib(n - 2)
    }
}`)

let t = Date.now()
let res = fib(35);
console.log(res == 9227465 ?  "OK: " + res : "WRONG: " + res, "time:", Date.now()-t)