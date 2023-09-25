<img src="./img/altair.png" width="150" height="150">

## Descrição
Altair Spankabytes é um simples interpretador feito exclusivamente para rodar a linguagem criada para a rinha de compiladores. Ele é baseado em tree-walking interpreter e é feito apenas em Go 1.21 com suas libs internas.

## Funcionalidades
- [x] Shadowing
- [x] Usa int64 como padrão e faz conversão automática em runtime para bigint (suportando valores altos no fibonacci com recursão de cauda).
- [x] Descreve erros em runtime indicando a linha/coluna e o código do trecho problemático.
- [x] Suporta recursões profundas.

## Benchmark (sem cache)
Intel(R) Core(TM) i5-9600KF CPU @ 3.70GHz

|Exemplo           | Altair           | NodeJS 18.12.0 (com eval) | Python 3.11.0     |
|:-----------------|:-----------------|:--------------------------|:-----------------:|
|factorial(10)     |~0.34 secs        |stack overflow             |~0.06 secs (sys.setrecursionlimit(10**6)) |
|fib(35)           |~0.9 secs :trophy:|~2.1 secs                  |~1.68 secs         |
|hanoi(25)         |~1.4 secs :trophy:|~2.4 secs                  |~2.2 secs          |
|combination(50, 5)|~0.37 secs        |~0.29 secs                 |~0.36 secs         |
|sum(50000)        |~0.1 secs         |~0.008 secs (--stack_size=5000) | ~0.007 secs (sys.setrecursionlimit(10**6)) |

## Como utilizar
```
go run . ./examples/fib.json
```

## Como testar
Dependências:
```
cargo install rinha
```
Execução dos testes:
```
go test -v ./interpreter
```
ou 
```
go run . ./examples/my-test.rinha
```