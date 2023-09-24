eval(`function hanoi(n){
    if(n == 1){
        return 1
    } else {
        return hanoi(n-1) + hanoi(n-1) + 1
    }
}`)

let t = Date.now()
let res = hanoi(25);
console.log(res == 33554431 ?  "OK: " + res : "WRONG: " + res, "time:", Date.now()-t)