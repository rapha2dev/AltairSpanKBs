eval(`function combination(n, k){
    let a = k == 0;
    let b = k == n;
    if (a || b){
        return 1
    }else{
        return combination(n - 1, k - 1) + combination(n - 1, k)
    }
}`)
let t = Date.now()
let res = combination(50, 5);
console.log(res == 2118760 ?  "OK: " + res : "WRONG: " + res, "time:", Date.now()-t)