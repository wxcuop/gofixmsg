Pyfixmsg_plus
========

![Lifecycle:Experimental](https://img.shields.io/badge/Lifecycle-Experimental-339999)

``pyfixmsg_plus``is a library for parsing, manipulating and serialising [FIX](http://www.fixtradingcommunity.org)
messages, primarily geared towards testing forked from https://github.com/morganstanley/pyfixmsg with experimental fixsession management supporting TCP and SSL fix sessions

Objectives
-----------
 * provide a rich API to compare and manipulate messages. 
 * (mostly) Message type agnostic,
 * (mostly) value types agnostic
 * pluggable : load specification XML files, custom specifications or build your own Specification class for repeating
 groups definitions and message types, define your own codec for custom serialisation or deserialisation quirks.


Dependencies
------------
 * ``six`` library (at least version 1.16.0).
 * python 3.11, 3.12, 3.13
 * Optional [lxml](http://lxml.de) for faster parsing of xml specification files.
 * Optional pytest, pytest-timeout to run the tests.
 * Optional [spec files from quickfix](https://github.com/quickfix/quickfix/tree/master/spec) to get started with 
 standard FIX specifications.
 
 
Core classes
------------
 * `FixMessage`. Inherits from ``dict``. Workhorse class. By default comes with a codec that will parse standard-looking
 ``FIX``, but without support repeating groups.
 * `Codec` defines how to parse a buffer into a FixMessage, and how to serialise it back
 * `Spec` defines the ``FIX`` specification to follow. Only required for support of repeating group. Defined from 
 Quickfix's spec XML files.
 

How to run the tests
--------------------
 * `` pytest --spec=/var/tmp/FIX44.xml `` will launch the tests against the spec file in /var/tmp. You will need to load
 the [spec files from quickfix](https://github.com/quickfix/quickfix/tree/master/spec) to get the tests to work. 
 The spec files are not included in this distribution.


Notes
-----
This is  a FIX message library that includes simple  FIX session management system.  It does not contain an order management 
core. It is purely message parsing-manipulation-serialisation and focused on testing. 

More documentation
------------------
Read the [documentation](http://pyfixmsg.readthedocs.io/), or browse the [examples](examples/pyfixmsg_example.py) file for 
many examples

* https://wxcuop.github.io/pyfixmsg_plus/index.html

Go-First Migration Guide
------------------------
For users migrating from the Python implementation to the Go-based rewrite (`gorewrite`), please see the [Go-First Migration Guide](gorewrite/doc/migration.md).
 
