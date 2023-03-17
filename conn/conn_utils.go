package conn

import "errors"

// ParseOptions 为了简化用户调用，通过 ...*Option 将 Option 实现为可选参数
func ParseOptions(opts ...*Option) (*Option, error) {
	// if opts is nil or pass nil as parameter
	if len(opts) == 0 || opts[0] == nil {
		return DefaultOption, nil
	}
	if len(opts) != 1 {
		return nil, errors.New("number of options is more than 1")
	}

	opt := opts[0]
	opt.MagicNumber = DefaultOption.MagicNumber
	if opt.ConnType == "" {
		opt.ConnType = DefaultOption.ConnType
	}

	return opt, nil
}
