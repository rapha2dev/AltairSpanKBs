eval(`function mult(a, b, c) {
    if(c > 1){                        
        return mult(a, b + a, c - 1)
    }else{
        return b
    }    
}
function factorial(n){    
    if (n == 1) {
        return n
    }else{               
        return mult(n, n, factorial(n - 1))        
    }
}`)
let t = Date.now()
let res = factorial(10)
console.log(res == 3628800 ?  "OK: " + res : "WRONG: " + res, "time:", Date.now()-t)