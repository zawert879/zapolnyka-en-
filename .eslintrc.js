module.exports = {
	env: {
		browser: true,
		es2021: true,
	},
	extends: 'xo',
	overrides: [
		{
			env: {
				node: true,
			},
			files: [
				'.eslintrc.{js,cjs}',
			],
			parserOptions: {
				sourceType: 'script',
			},
		},
		{
			extends: [
				'xo-typescript',
			],
			files: [
				'*.ts',
				'*.tsx',
			],
			rules: {
				semi: ['error', 'never'],
				'@typescript-eslint/semi': 'off',
				'@typescript-eslint/object-curly-spacing': ['error', 'always'],
				'no-unexpected-multiline': 'error',
				'@typescript-eslint/indent': ['error', 2],
				'@typescript-eslint/ban-types': 'off',
				'@typescript-eslint/prefer-readonly': 'off',
				'capitalized-comments': 'off',
				'no-implicit-coercion': 'off',
			},

		},
	],
	parserOptions: {
		ecmaVersion: 'latest',
		sourceType: 'module',
	},
	rules: {
		'object-curly-spacing': 'off',
	},
};
