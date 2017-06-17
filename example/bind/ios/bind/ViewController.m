// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

#import "ViewController.h"
@import Matcha;

@interface ViewController ()
@end

@implementation ViewController

@synthesize textLabel;

- (void)loadView {
    [super loadView];
    
    [MatchaObjcBridge sharedBridge].root = @"Fupo";
    MatchaGoValue *goRoot = [MatchaGoBridge sharedBridge].root;
    NSLog(@"%@", [goRoot call:@"TestMethod" args:nil][0].toString);
    NSLog(@"%@", [goRoot field:@"blah"].toString);
    
    textLabel.text = [goRoot field:@"blah"].toString;
    
    int test = MatchaTest();
    NSLog(@"%@", @(test));
}

@end
