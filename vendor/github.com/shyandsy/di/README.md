# di
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
![Test Status](https://github.com/shyandsy/di/actions/workflows/go-test.yml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/shyandsy/di)](https://goreportcard.com/report/github.com/shyandsy/di)
[![codecov](https://codecov.io/gh/shyandsy/di/graph/badge.svg?token=W8Z0SWZJG3)](https://codecov.io/gh/shyandsy/di)
[![CodeQL](https://github.com/shyandsy/di/actions/workflows/codeql.yml/badge.svg?branch=main&event=push)](https://github.com/shyandsy/di/actions/workflows/codeql.yml) 

A reflection based (DI)Dependency Injector component for golang project.

## Features
- [x] Provide: struct pointer or function  
- [x] ProvideAs: struct pointer or function as interface type
- [x] Find: dependency inject for struct pointer or interface
- [x] Resolve: dependency inject for struct fields
- [x] Invoke: use dependencies as parameter on invoke method
- [x] support recursive construct dependencies on find/resolve/invoke

## Installation
installation
```
go install github.com/shyandsy/di
```

## Usage
please check unit test code

create a container
```go
c := di.NewContainer()
```

### Provide
add a dependency which is a pointer of struct to container
```go
// object: dependency object, should be a pointer of struct 
Provide(object interface{}) error
```

usage:
```go
cat1 := &Cat{Name: "A"}
dog1 := &Dog{Name: "B"}

// Provide
err := c.Provide(cat1)
err = c.Provide(dog1)

cat2 := &Cat{}
dog2 := &Dog{}
	
// Find：cat2.Name = "A"
err = c.Find(cat2)

// Find：dog2.Name = "B"
err = c.Find(dog2)
```

### ProvideAs
we wanna programming to the interface rather than implementation.
ProvideAs is what you need to add dependency as an interface to the container.

```go
// object: dependency object, should be a pointer of struct 
// target: specify the interface type, should be a pointer of interface, egg: (*Pet)(nil)
ProvideAs(object interface{}, target interface{}) error
```

usage:
```go
type Animal interface {
    GetName() string
}

type Pet interface {
    Animal
}

cat1 := NewPetCat("cat")
animalCat := NewAnimalCat("ccc")

// Provide and ProvideAs
err = c.ProvideAs(cat1, (*Pet)(nil))
err = c.ProvideAs(animalCat, (*Animal)(nil))

var P Pet
var A Animal

// Find interface
// P.GetName(): "cat" 
err = c.Find(&P) 

// Find interface
// A.GetName(): "ccc"
err = c.Find(&A) 
```

### Find
find is to fetch a single dependency object

```go
// object: to fetch the dependency, should be a pointer of struct, or a pointer of interface  
Find(object interface{}) error
```

usage
```go
c := NewContainer()

var s Animal

// inject Animal interface
err = c.ProvideAs(&Cat{Name: "A"}, (*Animal)(nil))
assert.Nil(t, err)

// inject pointer of struct
err := c.Provide(&Cat{Name: "A"})
assert.Nil(t, err)

// use case 1: fetch pointer of struct 
a := Cat{}
err = c.Find(&a)
assert.Nil(t, err)
assert.Equal(t, a.GetName(), "A")

// use case 2: fetch interface
err = c.Find(&s)
assert.Nil(t, err)
assert.True(t, s != nil)
assert.Equal(t, s.GetName(), "A")
```

### Resolve
sometimes, we wanna simple inject dependencies based on struct field
- field type must be pointer of struct, or interface
- field must have `inject` tag
- field must be writable which is 

```go
// object: struct we wanna inject dependencies, object must be pointer of struct 
Resolve(object interface{}) error
```

usage:
```go
type Cat struct {
    Name string
}

type temp struct {
    Cat    *Cat   `inject:""`
    Pet    Pet    `inject:""`
    Animal Animal `inject:""`
}

c := NewContainer()

// Provide dependency
err := c.Provide(&Cat{Name: "A"})
err = c.ProvideAs(&Cat{Name: "B"}, (*Pet)(nil))
err = c.ProvideAs(NewPetCat("C"), (*Animal)(nil))
	
// Resolve struct field
s := &temp{}
err = c.Resolve(s)
assert.Nil(t, err)

/* 
now we have:
    s.Cat.GetName(): "A"
    s.Pet.GetName(): "B"
    s.Animal.GetName(): "C"
*/
```

### Invoke
call a function with dependencies injection

notice: the parameters of function called by Invoke must be pointer of struct, or interface
```go
// f: the function we wanna call
Invoke(f interface{}) error
```

usage
```go
func PrintCatAndAnimal(cat1 *Cat, cat2 *Cat, animal Animal) {
	fmt.Println(fmt.Sprintf("cat1:%s\ncat2:%s\nanimal:%s\n", cat1.Name, cat2.Name, animal.GetName()))
}

func PrintCatAndAnimalRecursive(s *temp2) {
    fmt.Println(fmt.Sprintf("cat1:%s\ncat2:%s\nanimal:%s\n", s.Temp.Cat.Name, s.Temp.Pet.GetName(), s.Temp.Animal.GetName()))
}

c := NewContainer()

cat := &Cat{Name: "A"}
animal := NewAnimalCat("B")

// add dependencies
err := c.Provide(cat)
assert.Nil(t, err)
err = c.ProvideAs(animal, (*Animal)(nil))
assert.Nil(t, err)
    
// invoke
err = c.Invoke(PrintCat)
assert.NotNil(t, err)

// invoke
err = c.Invoke(PrintCatWithInvalidParameter)
assert.NotNil(t, err)

// invoke
err = c.Invoke(PrintCatPointer)
assert.Nil(t, err)

// invoke
err = c.Invoke(PrintCatAndAnimal)
assert.Nil(t, err)
}
```

## Example
please check unit test
