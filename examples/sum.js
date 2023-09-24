eval(`function sum(n){
    if (n == 1) {
        return n
    } else {
        return n + sum(n - 1)
    }
}`)
let t = Date.now()
let res = sum(50000);
console.log(res == 1250025000 ?  "OK: " + res : "WRONG: " + res, "time:", Date.now()-t)